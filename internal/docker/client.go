package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

type Client struct{}

type ContainerSpec struct {
	ImageRef           string
	ContainerName      string
	Env                []string
	Labels             map[string]string
	WorkspacePath      string
	BootstrapMountPath string
	VolumeName         string
	DataMountPath      string
	NetworkName        string
}

type ContainerState struct {
	ContainerID   string
	ContainerName string
	Running       bool
	HealthStatus  string
}

type inspectRecord struct {
	ID    string `json:"Id"`
	State struct {
		Running bool   `json:"Running"`
		Status  string `json:"Status"`
		Health  *struct {
			Status string `json:"Status"`
		} `json:"Health"`
	} `json:"State"`
}

func New() (*Client, error) {
	return &Client{}, nil
}

func (c *Client) Ping(ctx context.Context) error {
	_, err := c.run(ctx, "docker", "version", "--format", "{{.Server.Version}}")
	if err != nil {
		return fmt.Errorf("docker ping: %w", err)
	}
	return nil
}

func (c *Client) EnsureNetwork(ctx context.Context, networkName string) error {
	if strings.TrimSpace(networkName) == "" || networkName == "bridge" {
		return nil
	}
	_, err := c.run(ctx, "docker", "network", "inspect", networkName)
	if err == nil {
		return nil
	}
	_, err = c.run(ctx, "docker", "network", "create", networkName)
	if err != nil {
		return fmt.Errorf("create network: %w", err)
	}
	return nil
}

func (c *Client) EnsureVolume(ctx context.Context, volumeName string) error {
	if strings.TrimSpace(volumeName) == "" {
		return nil
	}
	_, err := c.run(ctx, "docker", "volume", "inspect", volumeName)
	if err == nil {
		return nil
	}
	_, err = c.run(ctx, "docker", "volume", "create", volumeName)
	if err != nil {
		return fmt.Errorf("create volume: %w", err)
	}
	return nil
}

func (c *Client) RemoveVolume(ctx context.Context, volumeName string) error {
	if strings.TrimSpace(volumeName) == "" {
		return nil
	}
	_, err := c.run(ctx, "docker", "volume", "rm", "-f", volumeName)
	if err != nil && !isNoSuchObject(err) {
		return fmt.Errorf("remove volume: %w", err)
	}
	return nil
}

func (c *Client) CreateAndStartContainer(ctx context.Context, spec ContainerSpec, pollInterval time.Duration, timeout time.Duration) (ContainerState, error) {
	if err := c.EnsureVolume(ctx, spec.VolumeName); err != nil {
		return ContainerState{}, err
	}
	if err := c.RemoveContainer(ctx, spec.ContainerName); err != nil {
		return ContainerState{}, err
	}

	args := []string{"run", "-d", "--name", spec.ContainerName, "--network", "host"}
	if strings.TrimSpace(spec.WorkspacePath) != "" && strings.TrimSpace(spec.BootstrapMountPath) != "" {
		args = append(args, "-v", spec.WorkspacePath+":"+spec.BootstrapMountPath)
	}
	if strings.TrimSpace(spec.VolumeName) != "" && strings.TrimSpace(spec.DataMountPath) != "" {
		args = append(args, "-v", spec.VolumeName+":"+spec.DataMountPath)
	}
	for key, value := range spec.Labels {
		args = append(args, "--label", key+"="+value)
	}
	for _, envVar := range spec.Env {
		args = append(args, "-e", envVar)
	}
	args = append(args, spec.ImageRef)

	output, err := c.run(ctx, "docker", args...)
	if err != nil {
		return ContainerState{}, fmt.Errorf("create and start container: %w", err)
	}
	containerID := strings.TrimSpace(output)
	return c.WaitForHealthy(ctx, containerID, spec.ContainerName, pollInterval, timeout)
}

func (c *Client) StartContainer(ctx context.Context, containerRef string, pollInterval time.Duration, timeout time.Duration) (ContainerState, error) {
	if _, err := c.run(ctx, "docker", "start", containerRef); err != nil {
		return ContainerState{}, fmt.Errorf("start container: %w", err)
	}
	return c.WaitForHealthy(ctx, containerRef, containerRef, pollInterval, timeout)
}

func (c *Client) StopContainer(ctx context.Context, containerRef string) error {
	if strings.TrimSpace(containerRef) == "" {
		return nil
	}
	_, err := c.run(ctx, "docker", "stop", "-t", "10", containerRef)
	if err != nil && !isNoSuchObject(err) {
		return fmt.Errorf("stop container: %w", err)
	}
	return nil
}

func (c *Client) RestartContainer(ctx context.Context, containerRef string, pollInterval time.Duration, timeout time.Duration) (ContainerState, error) {
	if _, err := c.run(ctx, "docker", "restart", "-t", "10", containerRef); err != nil {
		return ContainerState{}, fmt.Errorf("restart container: %w", err)
	}
	return c.WaitForHealthy(ctx, containerRef, containerRef, pollInterval, timeout)
}

func (c *Client) RemoveContainer(ctx context.Context, containerRef string) error {
	if strings.TrimSpace(containerRef) == "" {
		return nil
	}
	_, err := c.run(ctx, "docker", "rm", "-f", containerRef)
	if err != nil && !isNoSuchObject(err) {
		return fmt.Errorf("remove container: %w", err)
	}
	return nil
}

func (c *Client) WaitForHealthy(ctx context.Context, containerRef string, containerName string, pollInterval time.Duration, timeout time.Duration) (ContainerState, error) {
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		record, err := c.inspectContainer(ctx, containerRef)
		if err == nil {
			health := "unknown"
			if record.State.Health != nil {
				health = record.State.Health.Status
			}
			if record.State.Running && (record.State.Health == nil || record.State.Health.Status == "healthy") {
				return ContainerState{ContainerID: record.ID, ContainerName: containerName, Running: true, HealthStatus: health}, nil
			}
			if record.State.Status == "exited" || record.State.Status == "dead" {
				return ContainerState{}, fmt.Errorf("container exited with status %s", record.State.Status)
			}
		}

		select {
		case <-ctx.Done():
			return ContainerState{}, ctx.Err()
		case <-deadline.C:
			return ContainerState{}, fmt.Errorf("timeout waiting for container health")
		case <-ticker.C:
		}
	}
}

func (c *Client) inspectContainer(ctx context.Context, containerRef string) (inspectRecord, error) {
	output, err := c.run(ctx, "docker", "inspect", containerRef)
	if err != nil {
		return inspectRecord{}, err
	}
	var records []inspectRecord
	if err := json.Unmarshal([]byte(output), &records); err != nil {
		return inspectRecord{}, fmt.Errorf("decode docker inspect: %w", err)
	}
	if len(records) == 0 {
		return inspectRecord{}, fmt.Errorf("empty inspect result")
	}
	return records[0], nil
}

func (c *Client) run(ctx context.Context, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%s %s: %w: %s", name, strings.Join(args, " "), err, strings.TrimSpace(string(output)))
	}
	return string(output), nil
}

func isNoSuchObject(err error) bool {
	return strings.Contains(err.Error(), "No such") || strings.Contains(err.Error(), "not found")
}
