package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	API      APIConfig      `yaml:"api"`
	Database DatabaseConfig `yaml:"database"`
	Runtime  RuntimeConfig  `yaml:"runtime"`
	Security SecurityConfig `yaml:"security"`
	Worker   WorkerConfig   `yaml:"worker"`
}

type APIConfig struct {
	ListenAddr string `yaml:"listen_addr"`
}

type DatabaseConfig struct {
	URL string `yaml:"url"`
}

type RuntimeConfig struct {
	WorkspaceRoot      string `yaml:"workspace_root"`
	DockerNetwork      string `yaml:"docker_network"`
	ImageRegistryPrefix string `yaml:"image_registry_prefix"`
	BootstrapMountPath string `yaml:"bootstrap_mount_path"`
	DataMountPath      string `yaml:"data_mount_path"`
	BaseDomain         string `yaml:"base_domain"`
	HealthPollInterval string `yaml:"health_poll_interval"`
	HealthTimeout      string `yaml:"health_timeout"`
}

type SecurityConfig struct {
	MasterKey   string `yaml:"master_key"`
	StaticToken string `yaml:"static_token"`
}

type WorkerConfig struct {
	Name              string `yaml:"name"`
	PollInterval      string `yaml:"poll_interval"`
	HeartbeatTTL      string `yaml:"heartbeat_ttl"`
	JobHeartbeatTTL   string `yaml:"job_heartbeat_ttl"`
}

func Default() Config {
	return Config{
		API: APIConfig{
			ListenAddr: ":8080",
		},
		Runtime: RuntimeConfig{
			WorkspaceRoot:      "/tmp/openclaw-autodeploy/tenants",
			DockerNetwork:      "bridge",
			ImageRegistryPrefix: "",
			BootstrapMountPath: "/bootstrap",
			DataMountPath:      "/data",
			BaseDomain:         "",
			HealthPollInterval: "3s",
			HealthTimeout:      "60s",
		},
		Security: SecurityConfig{
			StaticToken: "",
		},
		Worker: WorkerConfig{
			Name:         "ultraworker",
			PollInterval: "15s",
			HeartbeatTTL: "45s",
		},
	}
}

func Load(path string) (Config, error) {
	cfg := Default()
	if path != "" {
		contents, err := os.ReadFile(path)
		if err != nil {
			return Config{}, fmt.Errorf("read config: %w", err)
		}
		if err := yaml.Unmarshal(contents, &cfg); err != nil {
			return Config{}, fmt.Errorf("parse config: %w", err)
		}
	}
	applyEnv(&cfg)
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c Config) Validate() error {
	if strings.TrimSpace(c.API.ListenAddr) == "" {
		return fmt.Errorf("api.listen_addr is required")
	}
	if strings.TrimSpace(c.Database.URL) == "" {
		return fmt.Errorf("database.url is required")
	}
	if strings.TrimSpace(c.Runtime.WorkspaceRoot) == "" {
		return fmt.Errorf("runtime.workspace_root is required")
	}
	if strings.TrimSpace(c.Runtime.DockerNetwork) == "" {
		return fmt.Errorf("runtime.docker_network is required")
	}
	if strings.TrimSpace(c.Runtime.BootstrapMountPath) == "" {
		return fmt.Errorf("runtime.bootstrap_mount_path is required")
	}
	if strings.TrimSpace(c.Runtime.DataMountPath) == "" {
		return fmt.Errorf("runtime.data_mount_path is required")
	}
	if strings.TrimSpace(c.Security.MasterKey) == "" {
		return fmt.Errorf("security.master_key is required")
	}
	if _, err := c.RuntimeHealthPollInterval(); err != nil {
		return err
	}
	if _, err := c.RuntimeHealthTimeout(); err != nil {
		return err
	}
	if _, err := c.WorkerPollInterval(); err != nil {
		return err
	}
	if _, err := c.WorkerHeartbeatTTL(); err != nil {
		return err
	}
	if strings.TrimSpace(c.Worker.Name) == "" {
		return fmt.Errorf("worker.name is required")
	}
	return nil
}

func (c Config) WorkerPollInterval() (time.Duration, error) {
	d, err := time.ParseDuration(c.Worker.PollInterval)
	if err != nil {
		return 0, fmt.Errorf("parse worker.poll_interval: %w", err)
	}
	return d, nil
}

func (c Config) RuntimeHealthPollInterval() (time.Duration, error) {
	d, err := time.ParseDuration(c.Runtime.HealthPollInterval)
	if err != nil {
		return 0, fmt.Errorf("parse runtime.health_poll_interval: %w", err)
	}
	return d, nil
}

func (c Config) RuntimeHealthTimeout() (time.Duration, error) {
	d, err := time.ParseDuration(c.Runtime.HealthTimeout)
	if err != nil {
		return 0, fmt.Errorf("parse runtime.health_timeout: %w", err)
	}
	return d, nil
}

func (c Config) WorkerHeartbeatTTL() (time.Duration, error) {
	d, err := time.ParseDuration(c.Worker.HeartbeatTTL)
	if err != nil {
		return 0, fmt.Errorf("parse worker.heartbeat_ttl: %w", err)
	}
	return d, nil
}

func (c Config) WorkerJobHeartbeatTTL() (time.Duration, error) {
	d, err := time.ParseDuration(c.Worker.JobHeartbeatTTL)
	if err != nil {
		return 0, fmt.Errorf("parse worker.job_heartbeat_ttl: %w", err)
	}
	return d, nil
}

func applyEnv(cfg *Config) {
	override(&cfg.API.ListenAddr, "OPENCLAW_API_LISTEN_ADDR")
	override(&cfg.Database.URL, "OPENCLAW_DATABASE_URL")
	override(&cfg.Runtime.WorkspaceRoot, "OPENCLAW_RUNTIME_WORKSPACE_ROOT")
	override(&cfg.Runtime.DockerNetwork, "OPENCLAW_RUNTIME_DOCKER_NETWORK")
	override(&cfg.Runtime.ImageRegistryPrefix, "OPENCLAW_RUNTIME_IMAGE_REGISTRY_PREFIX")
	override(&cfg.Runtime.BootstrapMountPath, "OPENCLAW_RUNTIME_BOOTSTRAP_MOUNT_PATH")
	override(&cfg.Runtime.DataMountPath, "OPENCLAW_RUNTIME_DATA_MOUNT_PATH")
	override(&cfg.Runtime.BaseDomain, "OPENCLAW_RUNTIME_BASE_DOMAIN")
	override(&cfg.Runtime.HealthPollInterval, "OPENCLAW_RUNTIME_HEALTH_POLL_INTERVAL")
	override(&cfg.Runtime.HealthTimeout, "OPENCLAW_RUNTIME_HEALTH_TIMEOUT")
	override(&cfg.Security.MasterKey, "OPENCLAW_SECURITY_MASTER_KEY")
	override(&cfg.Security.StaticToken, "OPENCLAW_SECURITY_STATIC_TOKEN")
	override(&cfg.Worker.Name, "OPENCLAW_WORKER_NAME")
	override(&cfg.Worker.PollInterval, "OPENCLAW_WORKER_POLL_INTERVAL")
	override(&cfg.Worker.HeartbeatTTL, "OPENCLAW_WORKER_HEARTBEAT_TTL")
	override(&cfg.Worker.JobHeartbeatTTL, "OPENCLAW_WORKER_JOB_HEARTBEAT_TTL")
}

func override(target *string, key string) {
	value, ok := os.LookupEnv(key)
	if ok && strings.TrimSpace(value) != "" {
		*target = value
	}
}
