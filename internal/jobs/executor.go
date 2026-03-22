package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"openclaw-autodeploy/internal/config"
	internaldocker "openclaw-autodeploy/internal/docker"
	"openclaw-autodeploy/internal/domain"
	storepkg "openclaw-autodeploy/internal/store/postgres"

	"github.com/jackc/pgx/v5"
)

type Executor struct {
	cfg              config.Config
	store            *storepkg.Store
	runtime          *internaldocker.Client
	hostname         string
	pollInterval     time.Duration
	healthTimeout    time.Duration
	workerName       string
	jobHeartbeatTTL  time.Duration
}

func NewExecutor(cfg config.Config, store *storepkg.Store, runtime *internaldocker.Client) (*Executor, error) {
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}
	pollInterval, err := cfg.RuntimeHealthPollInterval()
	if err != nil {
		return nil, err
	}
	healthTimeout, err := cfg.RuntimeHealthTimeout()
	if err != nil {
		return nil, err
	}
	jobHeartbeatTTL, err := cfg.WorkerJobHeartbeatTTL()
	if err != nil {
		jobHeartbeatTTL = 30 * time.Second
	}
	return &Executor{cfg: cfg, store: store, runtime: runtime, hostname: hostname, pollInterval: pollInterval, healthTimeout: healthTimeout, workerName: cfg.Worker.Name, jobHeartbeatTTL: jobHeartbeatTTL}, nil
}

func (e *Executor) ProcessOnce(ctx context.Context) (bool, error) {
	job, err := e.store.ClaimNextPendingJob(ctx, e.workerName, e.jobHeartbeatTTL)
	if err != nil {
		if err == pgx.ErrNoRows {
			return false, nil
		}
		return false, err
	}

	if err := e.execute(ctx, job); err != nil {
		_ = e.store.MarkJobFailed(ctx, job.ID, err.Error())
		return true, err
	}
	_ = e.store.UpdateJobHeartbeat(ctx, job.ID)
	if err := e.store.MarkJobSucceeded(ctx, job.ID); err != nil {
		return true, err
	}
	return true, nil
}

func (e *Executor) execute(ctx context.Context, job domain.DeploymentJob) error {
	var action domain.DeploymentActionRequest
	if len(strings.TrimSpace(string(job.Payload))) > 0 {
		if err := json.Unmarshal(job.Payload, &action); err != nil {
			return fmt.Errorf("decode job payload: %w", err)
		}
	}

	switch job.JobType {
	case "deploy", "redeploy":
		return e.handleDeploy(ctx, job, action)
	case "stop":
		return e.handleStop(ctx, job)
	case "start":
		return e.handleStart(ctx, job)
	case "restart":
		return e.handleRestart(ctx, job)
	case "destroy":
		return e.handleDestroy(ctx, job, action)
	default:
		return fmt.Errorf("unsupported job type %q", job.JobType)
	}
}

func (e *Executor) handleDeploy(ctx context.Context, job domain.DeploymentJob, action domain.DeploymentActionRequest) error {
	tenant, err := e.store.GetTenant(ctx, job.TenantID)
	if err != nil {
		return fmt.Errorf("get tenant: %w", err)
	}
	profile, err := e.store.GetTenantProfile(ctx, job.TenantID)
	if err != nil {
		return fmt.Errorf("get tenant profile: %w", err)
	}
	if !profile.Validation.IsValid {
		return fmt.Errorf("tenant profile is invalid")
	}

	current, err := e.store.GetCurrentTenantInstance(ctx, job.TenantID)
	hasCurrent := err == nil && current.ContainerID != ""

	image, err := e.store.ResolveImageForTemplate(ctx, profile.TemplateID)
	if err != nil {
		return fmt.Errorf("resolve image: %w", err)
	}
	image.ImageRef = normalizeImageRef(image.ImageRef, e.cfg.Runtime.ImageRegistryPrefix)
	secrets, err := e.store.ListActiveTenantSecretsValues(ctx, job.TenantID)
	if err != nil {
		return fmt.Errorf("load secrets: %w", err)
	}

	alloc, err := e.store.GetTenantLLMAllocation(ctx, job.TenantID)
	if err == nil && alloc.APIKeyID != "" {
		rawKey, err := e.store.DecryptLLMAPIKey(ctx, alloc.APIKeyID)
		if err == nil && rawKey != "" {
			provider, err := e.store.GetLLMProvider(ctx, alloc.ProviderID)
			if err == nil {
				envName := strings.ToUpper(provider.Name) + "_API_KEY"
				secrets = append(secrets, domain.DecryptedSecret{SecretKey: envName, SecretType: "llm_api_key", Value: rawKey})
			}
		}
	}

	configVersion, err := e.store.NextConfigVersion(ctx, job.TenantID)
	if err != nil {
		return fmt.Errorf("next config version: %w", err)
	}

	workspacePath, envVars, err := e.renderWorkspace(ctx, tenant, profile, image, secrets, configVersion)
	if err != nil {
		return err
	}

	containerName := containerNameForTenant(tenant)
	volumeName := volumeNameForTenant(tenant.ID)
	routeURL := routeURL(profile.RouteKey, e.cfg.Runtime.BaseDomain)
	instance, err := e.store.CreateTenantInstance(ctx, storepkg.CreateTenantInstanceParams{
		TenantID:        tenant.ID,
		DeploymentJobID: job.ID,
		ImageRef:        image.ImageRef,
		Status:          "creating",
		HostNode:        e.hostname,
		WorkspacePath:   workspacePath,
		VolumeName:      volumeName,
		RouteURL:        routeURL,
		HealthStatus:    "unknown",
		ConfigVersion:   configVersion,
		ContainerName:   containerName,
	})
	if err != nil {
		return fmt.Errorf("create tenant instance: %w", err)
	}

	_ = e.store.UpdateJobHeartbeat(ctx, job.ID)

	if hasCurrent {
		_ = e.runtime.StopContainer(ctx, current.ContainerID)
	}

	state, err := e.runtime.CreateAndStartContainer(ctx, internaldocker.ContainerSpec{
		ImageRef:           image.ImageRef,
		ContainerName:      containerName,
		Env:                envVars,
		Labels:             traefikLabels(profile.RouteKey, e.cfg.Runtime.BaseDomain, containerName),
		WorkspacePath:      workspacePath,
		BootstrapMountPath: e.cfg.Runtime.BootstrapMountPath,
		VolumeName:         volumeName,
		DataMountPath:      e.cfg.Runtime.DataMountPath,
		NetworkName:        e.cfg.Runtime.DockerNetwork,
	}, e.pollInterval, e.healthTimeout)
	if err != nil {
		_, _ = e.store.UpdateInstanceStatus(ctx, instance.ID, "failed", "unknown")
		if hasCurrent {
			_, _ = e.runtime.StartContainer(ctx, current.ContainerID, e.pollInterval, e.healthTimeout)
			_, _ = e.store.UpdateInstanceStatus(ctx, current.ID, "running", "unknown")
		}
		return err
	}

	if hasCurrent {
		_ = e.runtime.RemoveContainer(ctx, current.ContainerID)
		_, _ = e.store.UpdateInstanceStatus(ctx, current.ID, "destroyed", "unknown")
	}

	if _, err := e.store.UpdateInstanceRunning(ctx, instance.ID, state.ContainerID, containerName, state.HealthStatus); err != nil {
		return fmt.Errorf("update instance running: %w", err)
	}
	if err := e.store.UpdateTenantStatus(ctx, tenant.ID, "running"); err != nil {
		return fmt.Errorf("update tenant status: %w", err)
	}
	_ = e.store.UpdateJobHeartbeat(ctx, job.ID)
	_ = action
	return nil
}

func (e *Executor) handleStop(ctx context.Context, job domain.DeploymentJob) error {
	_ = e.store.UpdateJobHeartbeat(ctx, job.ID)
	instance, err := e.store.GetCurrentTenantInstance(ctx, job.TenantID)
	if err != nil {
		return fmt.Errorf("get current instance: %w", err)
	}
	if err := e.runtime.StopContainer(ctx, containerRef(instance)); err != nil {
		return err
	}
	if _, err := e.store.UpdateInstanceStatus(ctx, instance.ID, "stopped", "unknown"); err != nil {
		return fmt.Errorf("update instance stopped: %w", err)
	}
	return e.store.UpdateTenantStatus(ctx, job.TenantID, "stopped")
}

func (e *Executor) handleStart(ctx context.Context, job domain.DeploymentJob) error {
	_ = e.store.UpdateJobHeartbeat(ctx, job.ID)
	instance, err := e.store.GetCurrentTenantInstance(ctx, job.TenantID)
	if err != nil {
		return fmt.Errorf("get current instance: %w", err)
	}
	state, err := e.runtime.StartContainer(ctx, containerRef(instance), e.pollInterval, e.healthTimeout)
	if err != nil {
		return err
	}
	if _, err := e.store.UpdateInstanceRunning(ctx, instance.ID, state.ContainerID, instance.ContainerName, state.HealthStatus); err != nil {
		return fmt.Errorf("update instance running: %w", err)
	}
	return e.store.UpdateTenantStatus(ctx, job.TenantID, "running")
}

func (e *Executor) handleRestart(ctx context.Context, job domain.DeploymentJob) error {
	_ = e.store.UpdateJobHeartbeat(ctx, job.ID)
	instance, err := e.store.GetCurrentTenantInstance(ctx, job.TenantID)
	if err != nil {
		return fmt.Errorf("get current instance: %w", err)
	}
	state, err := e.runtime.RestartContainer(ctx, containerRef(instance), e.pollInterval, e.healthTimeout)
	if err != nil {
		return err
	}
	if _, err := e.store.UpdateInstanceRunning(ctx, instance.ID, state.ContainerID, instance.ContainerName, state.HealthStatus); err != nil {
		return fmt.Errorf("update instance running: %w", err)
	}
	return e.store.UpdateTenantStatus(ctx, job.TenantID, "running")
}

func (e *Executor) handleDestroy(ctx context.Context, job domain.DeploymentJob, action domain.DeploymentActionRequest) error {
	_ = e.store.UpdateJobHeartbeat(ctx, job.ID)
	instance, err := e.store.GetCurrentTenantInstance(ctx, job.TenantID)
	if err != nil {
		return fmt.Errorf("get current instance: %w", err)
	}
	if err := e.runtime.RemoveContainer(ctx, containerRef(instance)); err != nil {
		return err
	}
	if action.DestroyVolume {
		if err := e.runtime.RemoveVolume(ctx, instance.VolumeName); err != nil {
			return err
		}
	}
	if action.DestroyWorkspace && strings.TrimSpace(instance.WorkspacePath) != "" {
		if err := os.RemoveAll(instance.WorkspacePath); err != nil {
			return fmt.Errorf("destroy workspace: %w", err)
		}
	}
	if _, err := e.store.UpdateInstanceStatus(ctx, instance.ID, "destroyed", "unknown"); err != nil {
		return fmt.Errorf("update instance destroyed: %w", err)
	}
	return e.store.UpdateTenantStatus(ctx, job.TenantID, "ready")
}

func (e *Executor) renderWorkspace(_ context.Context, tenant domain.Tenant, profile domain.TenantProfile, image domain.Image, secrets []domain.DecryptedSecret, configVersion int) (string, []string, error) {
	tenantRoot := filepath.Join(e.cfg.Runtime.WorkspaceRoot, tenant.ID)
	releasePath := filepath.Join(tenantRoot, "releases", strconv.Itoa(configVersion))
	configPath := filepath.Join(releasePath, "config")
	dataPath := filepath.Join(releasePath, "data")
	currentPath := filepath.Join(tenantRoot, "current")

	if err := os.MkdirAll(configPath, 0o755); err != nil {
		return "", nil, fmt.Errorf("create config directory: %w", err)
	}
	if err := os.MkdirAll(dataPath, 0o755); err != nil {
		return "", nil, fmt.Errorf("create data directory: %w", err)
	}

	envVars := []string{
		"OPENCLAW_TENANT_ID=" + tenant.ID,
		"OPENCLAW_EXTERNAL_USER_ID=" + tenant.ExternalUserID,
		"OPENCLAW_MODEL_PROVIDER=" + profile.ModelProvider,
		"OPENCLAW_MODEL_NAME=" + profile.ModelName,
		"OPENCLAW_ROUTE_KEY=" + profile.RouteKey,
		"OPENCLAW_TEMPLATE_ID=" + profile.TemplateID,
		"OPENCLAW_IMAGE_REF=" + image.ImageRef,
		"OPENCLAW_BOOTSTRAP_DIR=" + e.cfg.Runtime.BootstrapMountPath,
	}
	for _, secret := range secrets {
		envVars = append(envVars, secret.SecretKey+"="+secret.Value)
	}

	if err := os.WriteFile(filepath.Join(configPath, "channels.json"), profile.Channels, 0o600); err != nil {
		return "", nil, fmt.Errorf("write channels.json: %w", err)
	}
	if err := os.WriteFile(filepath.Join(configPath, "skills.json"), profile.Skills, 0o600); err != nil {
		return "", nil, fmt.Errorf("write skills.json: %w", err)
	}
	if err := os.WriteFile(filepath.Join(configPath, "extra_files.json"), profile.ExtraFiles, 0o600); err != nil {
		return "", nil, fmt.Errorf("write extra_files.json: %w", err)
	}
	if err := os.WriteFile(filepath.Join(configPath, "SOUL.md"), []byte(profile.SoulMarkdown), 0o600); err != nil {
		return "", nil, fmt.Errorf("write SOUL.md: %w", err)
	}
	if err := os.WriteFile(filepath.Join(configPath, "memory.md"), []byte(profile.MemoryMarkdown), 0o600); err != nil {
		return "", nil, fmt.Errorf("write memory.md: %w", err)
	}
	metadata, err := json.MarshalIndent(map[string]any{
		"tenant_id":      tenant.ID,
		"slug":           tenant.Slug,
		"config_version": configVersion,
		"template_id":    profile.TemplateID,
		"image_ref":      image.ImageRef,
		"route_key":      profile.RouteKey,
	}, "", "  ")
	if err != nil {
		return "", nil, fmt.Errorf("marshal metadata: %w", err)
	}
	if err := os.WriteFile(filepath.Join(configPath, "metadata.json"), metadata, 0o600); err != nil {
		return "", nil, fmt.Errorf("write metadata.json: %w", err)
	}
	if err := os.WriteFile(filepath.Join(configPath, "app.env"), []byte(strings.Join(envVars, "\n")+"\n"), 0o600); err != nil {
		return "", nil, fmt.Errorf("write app.env: %w", err)
	}

	_ = os.Remove(currentPath)
	if err := os.Symlink(releasePath, currentPath); err != nil {
		return "", nil, fmt.Errorf("update current symlink: %w", err)
	}
	return currentPath, envVars, nil
}

func containerRef(instance domain.TenantInstance) string {
	if strings.TrimSpace(instance.ContainerID) != "" {
		return instance.ContainerID
	}
	return instance.ContainerName
}

func containerNameForTenant(tenant domain.Tenant) string {
	base := tenant.Slug
	if strings.TrimSpace(base) == "" {
		base = tenant.ID
	}
	replacer := strings.NewReplacer("_", "-", "/", "-", " ", "-", ".", "-")
	return "openclaw-" + strings.ToLower(replacer.Replace(base))
}

func volumeNameForTenant(tenantID string) string {
	replacer := strings.NewReplacer("_", "-", "/", "-", " ", "-", ".", "-")
	return "tenant-" + strings.ToLower(replacer.Replace(tenantID)) + "-data"
}

func routeURL(routeKey string, baseDomain string) string {
	if strings.TrimSpace(routeKey) == "" {
		return ""
	}
	if strings.TrimSpace(baseDomain) == "" {
		return routeKey
	}
	return "https://" + routeKey + "." + strings.TrimSpace(baseDomain)
}

func traefikLabels(routeKey string, baseDomain string, serviceName string) map[string]string {
	labels := map[string]string{
		"managed.by": "openclaw-autodeploy",
		"service":    "tenant-runtime",
	}
	if strings.TrimSpace(routeKey) == "" || strings.TrimSpace(baseDomain) == "" {
		return labels
	}
	routerName := strings.ReplaceAll(serviceName, "/", "-")
	labels["traefik.enable"] = "true"
	labels["traefik.http.routers."+routerName+".rule"] = "Host(`" + routeKey + "." + baseDomain + "`)"
	labels["traefik.http.services."+routerName+".loadbalancer.server.port"] = "18789"
	return labels
}

func normalizeImageRef(imageRef string, registryPrefix string) string {
	trimmedRef := strings.TrimSpace(imageRef)
	trimmedPrefix := strings.Trim(strings.TrimSpace(registryPrefix), "/")
	if trimmedRef == "" || trimmedPrefix == "" {
		return trimmedRef
	}
	firstSegment := trimmedRef
	hasSlash := false
	if slash := strings.Index(trimmedRef, "/"); slash >= 0 {
		hasSlash = true
		firstSegment = trimmedRef[:slash]
	}
	if hasSlash && (strings.Contains(firstSegment, ".") || strings.Contains(firstSegment, ":") || firstSegment == "localhost") {
		return trimmedRef
	}
	return trimmedPrefix + "/" + trimmedRef
}
