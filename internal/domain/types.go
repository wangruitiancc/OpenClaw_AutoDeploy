package domain

import (
	"encoding/json"
	"time"
)

type Tenant struct {
	ID             string    `json:"id" yaml:"id"`
	ExternalUserID string    `json:"external_user_id" yaml:"external_user_id"`
	Slug           string    `json:"slug" yaml:"slug"`
	DisplayName    string    `json:"display_name" yaml:"display_name"`
	Status         string    `json:"status" yaml:"status"`
	CreatedAt      time.Time `json:"created_at" yaml:"created_at"`
	UpdatedAt      time.Time `json:"updated_at" yaml:"updated_at"`
}

type TenantFilter struct {
	Status         string
	ExternalUserID string
	Slug           string
	Page           int
	PageSize       int
}

type TenantProfile struct {
	TenantID         string           `json:"tenant_id" yaml:"tenant_id"`
	TemplateID       string           `json:"template_id" yaml:"template_id"`
	ResourceTier     string           `json:"resource_tier" yaml:"resource_tier"`
	RouteKey         string           `json:"route_key" yaml:"route_key"`
	ModelProvider    string           `json:"model_provider" yaml:"model_provider"`
	ModelName        string           `json:"model_name" yaml:"model_name"`
	Channels         json.RawMessage  `json:"channels" yaml:"channels"`
	Skills           json.RawMessage  `json:"skills" yaml:"skills"`
	SoulMarkdown     string           `json:"soul_markdown" yaml:"soul_markdown"`
	MemoryMarkdown   string           `json:"memory_markdown" yaml:"memory_markdown"`
	ExtraFiles       json.RawMessage  `json:"extra_files" yaml:"extra_files"`
	Validation       ValidationResult `json:"validation" yaml:"validation"`
	UpdatedAt        time.Time        `json:"updated_at" yaml:"updated_at"`
}

type ValidationIssue struct {
	Field   string `json:"field" yaml:"field"`
	Message string `json:"message" yaml:"message"`
}

type ValidationResult struct {
	IsValid bool              `json:"is_valid" yaml:"is_valid"`
	Errors  []ValidationIssue `json:"errors" yaml:"errors"`
}

type SecretMetadata struct {
	TenantID          string    `json:"tenant_id" yaml:"tenant_id"`
	SecretKey         string    `json:"secret_key" yaml:"secret_key"`
	SecretType        string    `json:"secret_type" yaml:"secret_type"`
	ValueFingerprint  string    `json:"value_fingerprint" yaml:"value_fingerprint"`
	Version           int       `json:"version" yaml:"version"`
	Status            string    `json:"status" yaml:"status"`
	CreatedAt         time.Time `json:"created_at" yaml:"created_at"`
	UpdatedAt         time.Time `json:"updated_at" yaml:"updated_at"`
	RevokedAt         time.Time `json:"revoked_at,omitempty" yaml:"revoked_at,omitempty"`
	HasRevokedAtValue bool      `json:"-" yaml:"-"`
}

type Template struct {
	ID              string          `json:"id" yaml:"id"`
	Code            string          `json:"code" yaml:"code"`
	Name            string          `json:"name" yaml:"name"`
	Description     string          `json:"description" yaml:"description"`
	BaseImagePolicy json.RawMessage `json:"base_image_policy" yaml:"base_image_policy"`
	DefaultChannels json.RawMessage `json:"default_channels" yaml:"default_channels"`
	DefaultSkills   json.RawMessage `json:"default_skills" yaml:"default_skills"`
	DefaultFiles    json.RawMessage `json:"default_files" yaml:"default_files"`
	ResourcePolicy  json.RawMessage `json:"resource_policy" yaml:"resource_policy"`
	Enabled         bool            `json:"enabled" yaml:"enabled"`
}

type Image struct {
	ID                string    `json:"id" yaml:"id"`
	ImageRef          string    `json:"image_ref" yaml:"image_ref"`
	Version           string    `json:"version" yaml:"version"`
	RuntimeFamily     string    `json:"runtime_family" yaml:"runtime_family"`
	Status            string    `json:"status" yaml:"status"`
	DefaultTemplateID string    `json:"default_template_id" yaml:"default_template_id"`
	CreatedAt         time.Time `json:"created_at" yaml:"created_at"`
	UpdatedAt         time.Time `json:"updated_at" yaml:"updated_at"`
}

type DeploymentJob struct {
	ID             string          `json:"id" yaml:"id"`
	TenantID       string          `json:"tenant_id" yaml:"tenant_id"`
	JobType        string          `json:"job_type" yaml:"job_type"`
	RequestedBy    string          `json:"requested_by" yaml:"requested_by"`
	IdempotencyKey string          `json:"idempotency_key,omitempty" yaml:"idempotency_key,omitempty"`
	Status         string          `json:"status" yaml:"status"`
	Payload        json.RawMessage `json:"payload,omitempty" yaml:"payload,omitempty"`
	AttemptCount   int             `json:"attempt_count" yaml:"attempt_count"`
	LastError      string          `json:"last_error,omitempty" yaml:"last_error,omitempty"`
	ScheduledAt    time.Time       `json:"scheduled_at" yaml:"scheduled_at"`
	StartedAt      *time.Time      `json:"started_at,omitempty" yaml:"started_at,omitempty"`
	FinishedAt     *time.Time      `json:"finished_at,omitempty" yaml:"finished_at,omitempty"`
	WorkerName     string          `json:"worker_name,omitempty" yaml:"worker_name,omitempty"`
	HeartbeatAt    *time.Time      `json:"heartbeat_at,omitempty" yaml:"heartbeat_at,omitempty"`
}

type DeploymentJobFilter struct {
	TenantID string
	JobType  string
	Status   string
	Page     int
	PageSize int
}

type TenantInstance struct {
	ID              string     `json:"id" yaml:"id"`
	TenantID        string     `json:"tenant_id" yaml:"tenant_id"`
	DeploymentJobID string     `json:"deployment_job_id,omitempty" yaml:"deployment_job_id,omitempty"`
	ContainerID     string     `json:"container_id,omitempty" yaml:"container_id,omitempty"`
	ContainerName   string     `json:"container_name,omitempty" yaml:"container_name,omitempty"`
	ImageRef        string     `json:"image_ref,omitempty" yaml:"image_ref,omitempty"`
	Status          string     `json:"status" yaml:"status"`
	HostNode        string     `json:"host_node,omitempty" yaml:"host_node,omitempty"`
	WorkspacePath   string     `json:"workspace_path,omitempty" yaml:"workspace_path,omitempty"`
	VolumeName      string     `json:"volume_name,omitempty" yaml:"volume_name,omitempty"`
	RouteURL        string     `json:"route_url,omitempty" yaml:"route_url,omitempty"`
	HealthStatus    string     `json:"health_status" yaml:"health_status"`
	ConfigVersion   int        `json:"config_version" yaml:"config_version"`
	StartedAt       *time.Time `json:"started_at,omitempty" yaml:"started_at,omitempty"`
	StoppedAt       *time.Time `json:"stopped_at,omitempty" yaml:"stopped_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at" yaml:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at" yaml:"updated_at"`
}

type DecryptedSecret struct {
	SecretKey  string `json:"secret_key" yaml:"secret_key"`
	SecretType string `json:"secret_type" yaml:"secret_type"`
	Value      string `json:"value" yaml:"value"`
}

type CreateTenantRequest struct {
	ExternalUserID string `json:"external_user_id"`
	Slug           string `json:"slug"`
	DisplayName    string `json:"display_name"`
}

type UpsertTenantProfileRequest struct {
	TemplateID     string          `json:"template_id"`
	ResourceTier   string          `json:"resource_tier"`
	RouteKey       string          `json:"route_key"`
	ModelProvider  string          `json:"model_provider"`
	ModelName      string          `json:"model_name"`
	Channels       json.RawMessage `json:"channels"`
	Skills         json.RawMessage `json:"skills"`
	SoulMarkdown   string          `json:"soul_markdown"`
	MemoryMarkdown string          `json:"memory_markdown"`
	ExtraFiles     json.RawMessage `json:"extra_files"`
}

type SetSecretRequest struct {
	Value      string `json:"value"`
	SecretType string `json:"secret_type"`
}

type DeploymentActionRequest struct {
	Reason           string `json:"reason"`
	Wait             bool   `json:"wait,omitempty"`
	Strategy         string `json:"strategy,omitempty"`
	DestroyWorkspace bool   `json:"destroy_workspace,omitempty"`
	DestroyVolume    bool   `json:"destroy_volume,omitempty"`
}

type HealthResponse struct {
	Status string            `json:"status" yaml:"status"`
	Checks map[string]string `json:"checks,omitempty" yaml:"checks,omitempty"`
}

type TenantResponse struct {
	Tenant Tenant `json:"tenant" yaml:"tenant"`
}

type TenantsResponse struct {
	Tenants []Tenant `json:"tenants" yaml:"tenants"`
}

type ProfileResponse struct {
	Profile TenantProfile `json:"profile" yaml:"profile"`
}

type ValidationResponse struct {
	Validation ValidationResult `json:"validation" yaml:"validation"`
}

type SecretResponse struct {
	Secret SecretMetadata `json:"secret" yaml:"secret"`
}

type SecretsResponse struct {
	Secrets []SecretMetadata `json:"secrets" yaml:"secrets"`
}

type TemplatesResponse struct {
	Templates []Template `json:"templates" yaml:"templates"`
}

type TemplateResponse struct {
	Template Template `json:"template" yaml:"template"`
}

type ImagesResponse struct {
	Images []Image `json:"images" yaml:"images"`
}

type DeploymentJobResponse struct {
	Job DeploymentJob `json:"job" yaml:"job"`
}

type DeploymentJobsResponse struct {
	Jobs []DeploymentJob `json:"jobs" yaml:"jobs"`
}

type TenantInstanceResponse struct {
	Instance TenantInstance `json:"instance" yaml:"instance"`
}

type TenantInstancesResponse struct {
	Instances []TenantInstance `json:"instances" yaml:"instances"`
}

func (p TenantProfile) ToUpsertRequest() UpsertTenantProfileRequest {
	return UpsertTenantProfileRequest{
		TemplateID:     p.TemplateID,
		ResourceTier:   p.ResourceTier,
		RouteKey:       p.RouteKey,
		ModelProvider:  p.ModelProvider,
		ModelName:      p.ModelName,
		Channels:       append(json.RawMessage(nil), p.Channels...),
		Skills:         append(json.RawMessage(nil), p.Skills...),
		SoulMarkdown:   p.SoulMarkdown,
		MemoryMarkdown: p.MemoryMarkdown,
		ExtraFiles:     append(json.RawMessage(nil), p.ExtraFiles...),
	}
}

type LLMProvider struct {
	ID          string    `json:"id" yaml:"id"`
	Name        string    `json:"name" yaml:"name"`
	DisplayName string    `json:"display_name" yaml:"display_name"`
	Description string    `json:"description" yaml:"description"`
	BaseURL     string    `json:"base_url" yaml:"base_url"`
	Status      string    `json:"status" yaml:"status"`
	CreatedAt   time.Time `json:"created_at" yaml:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" yaml:"updated_at"`
}

type LLMAPIKey struct {
	ID             string    `json:"id" yaml:"id"`
	ProviderID    string    `json:"provider_id" yaml:"provider_id"`
	ProviderName  string    `json:"provider_name" yaml:"provider_name"`
	KeyFingerprint string   `json:"key_fingerprint" yaml:"key_fingerprint"`
	AllocatedCount int      `json:"allocated_count" yaml:"allocated_count"`
	Status        string    `json:"status" yaml:"status"`
	CreatedAt     time.Time `json:"created_at" yaml:"created_at"`
	UpdatedAt     time.Time `json:"updated_at" yaml:"updated_at"`
}

type TenantLLMKeyAllocation struct {
	ID         string    `json:"id" yaml:"id"`
	TenantID   string    `json:"tenant_id" yaml:"tenant_id"`
	ProviderID string    `json:"provider_id" yaml:"provider_id"`
	APIKeyID   string    `json:"api_key_id" yaml:"api_key_id"`
	ModelName  string    `json:"model_name" yaml:"model_name"`
	CreatedAt  time.Time `json:"created_at" yaml:"created_at"`
}

type LLMProviderResponse struct {
	Provider LLMProvider `json:"provider" yaml:"provider"`
}

type LLMProvidersResponse struct {
	Providers []LLMProvider `json:"providers" yaml:"providers"`
}

type LLMAPIKeyResponse struct {
	APIKey LLMAPIKey `json:"api_key" yaml:"api_key"`
}

type LLMAPIKeysResponse struct {
	APIKeys []LLMAPIKey `json:"api_keys" yaml:"api_keys"`
}

type TenantLLMAllocationResponse struct {
	Allocation TenantLLMKeyAllocation `json:"allocation" yaml:"allocation"`
}

type AllocateLLMKeyRequest struct {
	APIKeyID  string `json:"api_key_id"`
	ModelName string `json:"model_name"`
}

type UpsertLLMProviderRequest struct {
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
	Description string `json:"description"`
	BaseURL     string `json:"base_url"`
	Status      string `json:"status"`
}

type AddLLMAPIKeyRequest struct {
	ProviderID string `json:"provider_id"`
	Value      string `json:"value"`
	Status     string `json:"status"`
}
