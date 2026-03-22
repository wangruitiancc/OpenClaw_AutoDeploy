package postgres

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"openclaw-autodeploy/internal/domain"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	pool      *pgxpool.Pool
	masterKey string
}

type CreateTenantInstanceParams struct {
	TenantID        string
	DeploymentJobID string
	ImageRef        string
	Status          string
	HostNode        string
	WorkspacePath   string
	VolumeName      string
	RouteURL        string
	HealthStatus    string
	ConfigVersion   int
	ContainerName   string
}

func New(pool *pgxpool.Pool, masterKey string) *Store {
	return &Store{pool: pool, masterKey: masterKey}
}

func (s *Store) Ping(ctx context.Context) error {
	return s.pool.Ping(ctx)
}

func (s *Store) CreateTenant(ctx context.Context, input domain.CreateTenantRequest) (domain.Tenant, error) {
	row := s.pool.QueryRow(ctx, `
		INSERT INTO tenants (external_user_id, slug, display_name, status)
		VALUES ($1, $2, $3, 'draft')
		RETURNING id::text, external_user_id, slug, display_name, status, created_at, updated_at
	`, strings.TrimSpace(input.ExternalUserID), strings.TrimSpace(input.Slug), strings.TrimSpace(input.DisplayName))
	return scanTenant(row)
}

func (s *Store) GetTenant(ctx context.Context, tenantID string) (domain.Tenant, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id::text, external_user_id, slug, display_name, status, created_at, updated_at
		FROM tenants
		WHERE id = $1
	`, tenantID)
	return scanTenant(row)
}

func (s *Store) ListTenants(ctx context.Context, filter domain.TenantFilter) ([]domain.Tenant, error) {
	page := filter.Page
	if page < 1 {
		page = 1
	}
	pageSize := filter.PageSize
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	offset := (page - 1) * pageSize

	rows, err := s.pool.Query(ctx, `
		SELECT id::text, external_user_id, slug, display_name, status, created_at, updated_at
		FROM tenants
		WHERE ($1 = '' OR status = $1)
		  AND ($2 = '' OR external_user_id = $2)
		  AND ($3 = '' OR slug = $3)
		ORDER BY created_at DESC
		LIMIT $4 OFFSET $5
	`, strings.TrimSpace(filter.Status), strings.TrimSpace(filter.ExternalUserID), strings.TrimSpace(filter.Slug), pageSize, offset)
	if err != nil {
		return nil, fmt.Errorf("list tenants: %w", err)
	}
	defer rows.Close()

	var tenants []domain.Tenant
	for rows.Next() {
		tenant, err := scanTenant(rows)
		if err != nil {
			return nil, err
		}
		tenants = append(tenants, tenant)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list tenants rows: %w", err)
	}
	return tenants, nil
}

func (s *Store) UpsertTenantProfile(ctx context.Context, tenantID string, input domain.UpsertTenantProfileRequest, validation domain.ValidationResult) (domain.TenantProfile, error) {
	channels, err := normalizeJSON(input.Channels, `[]`)
	if err != nil {
		return domain.TenantProfile{}, err
	}
	skills, err := normalizeJSON(input.Skills, `[]`)
	if err != nil {
		return domain.TenantProfile{}, err
	}
	extraFiles, err := normalizeJSON(input.ExtraFiles, `[]`)
	if err != nil {
		return domain.TenantProfile{}, err
	}
	validationErrors, err := json.Marshal(validation.Errors)
	if err != nil {
		return domain.TenantProfile{}, fmt.Errorf("marshal validation errors: %w", err)
	}

	row := s.pool.QueryRow(ctx, `
		INSERT INTO tenant_profiles (
			tenant_id, template_id, resource_tier, route_key, model_provider, model_name,
			channels, skills, soul_markdown, memory_markdown, extra_files,
			is_valid, validation_errors, updated_at
		)
		VALUES (
			$1, $2, $3, $4, $5, $6,
			$7::jsonb, $8::jsonb, $9, $10, $11::jsonb,
			$12, $13::jsonb, NOW()
		)
		ON CONFLICT (tenant_id) DO UPDATE SET
			template_id = EXCLUDED.template_id,
			resource_tier = EXCLUDED.resource_tier,
			route_key = EXCLUDED.route_key,
			model_provider = EXCLUDED.model_provider,
			model_name = EXCLUDED.model_name,
			channels = EXCLUDED.channels,
			skills = EXCLUDED.skills,
			soul_markdown = EXCLUDED.soul_markdown,
			memory_markdown = EXCLUDED.memory_markdown,
			extra_files = EXCLUDED.extra_files,
			is_valid = EXCLUDED.is_valid,
			validation_errors = EXCLUDED.validation_errors,
			updated_at = NOW()
		RETURNING
			tenant_id::text, template_id::text, resource_tier, route_key, model_provider, model_name,
			channels::text, skills::text, soul_markdown, memory_markdown, extra_files::text,
			is_valid, validation_errors::text, updated_at
	`, tenantID, input.TemplateID, input.ResourceTier, input.RouteKey, input.ModelProvider, input.ModelName,
		string(channels), string(skills), input.SoulMarkdown, input.MemoryMarkdown, string(extraFiles),
		validation.IsValid, string(validationErrors))
	return scanProfile(row)
}

func (s *Store) GetTenantProfile(ctx context.Context, tenantID string) (domain.TenantProfile, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT
			tenant_id::text, template_id::text, resource_tier, route_key, model_provider, model_name,
			channels::text, skills::text, soul_markdown, memory_markdown, extra_files::text,
			is_valid, validation_errors::text, updated_at
		FROM tenant_profiles
		WHERE tenant_id = $1
	`, tenantID)
	return scanProfile(row)
}

func (s *Store) TemplateExists(ctx context.Context, templateID string) (bool, error) {
	var exists bool
	err := s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM deployment_templates WHERE id = $1 AND enabled = TRUE)`, templateID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("template exists: %w", err)
	}
	return exists, nil
}

func (s *Store) RouteKeyInUse(ctx context.Context, tenantID string, routeKey string) (bool, error) {
	var exists bool
	err := s.pool.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM tenant_profiles
			WHERE route_key = $1 AND tenant_id <> $2
		)
	`, routeKey, tenantID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("route key exists: %w", err)
	}
	return exists, nil
}

func (s *Store) SecretExists(ctx context.Context, tenantID string, secretKey string) (bool, error) {
	var exists bool
	err := s.pool.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM tenant_secrets
			WHERE tenant_id = $1 AND secret_key = $2 AND status = 'active'
		)
	`, tenantID, secretKey).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("secret exists: %w", err)
	}
	return exists, nil
}

func (s *Store) SetTenantSecret(ctx context.Context, tenantID string, secretKey string, input domain.SetSecretRequest, fingerprint string) (domain.SecretMetadata, error) {
	secretType := strings.TrimSpace(input.SecretType)
	if secretType == "" {
		secretType = "api_key"
	}

	row := s.pool.QueryRow(ctx, `
		INSERT INTO tenant_secrets (
			tenant_id, secret_type, secret_key, encrypted_value, value_fingerprint,
			version, status, created_at, updated_at, revoked_at
		)
		VALUES (
			$1, $2, $3, pgp_sym_encrypt($4, $5), $6,
			1, 'active', NOW(), NOW(), NULL
		)
		ON CONFLICT (tenant_id, secret_key) DO UPDATE SET
			secret_type = EXCLUDED.secret_type,
			encrypted_value = pgp_sym_encrypt($4, $5),
			value_fingerprint = EXCLUDED.value_fingerprint,
			version = tenant_secrets.version + 1,
			status = 'active',
			updated_at = NOW(),
			revoked_at = NULL
		RETURNING tenant_id::text, secret_key, secret_type, value_fingerprint, version, status, created_at, updated_at, revoked_at
	`, tenantID, secretType, secretKey, input.Value, s.masterKey, fingerprint)
	return scanSecretMetadata(row)
}

func (s *Store) ListTenantSecrets(ctx context.Context, tenantID string) ([]domain.SecretMetadata, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT tenant_id::text, secret_key, secret_type, value_fingerprint, version, status, created_at, updated_at, revoked_at
		FROM tenant_secrets
		WHERE tenant_id = $1
		ORDER BY secret_key ASC
	`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("list tenant secrets: %w", err)
	}
	defer rows.Close()

	var secrets []domain.SecretMetadata
	for rows.Next() {
		secret, err := scanSecretMetadata(rows)
		if err != nil {
			return nil, err
		}
		secrets = append(secrets, secret)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list tenant secrets rows: %w", err)
	}
	return secrets, nil
}

func (s *Store) DeleteTenantSecret(ctx context.Context, tenantID string, secretKey string) (bool, error) {
	commandTag, err := s.pool.Exec(ctx, `
		UPDATE tenant_secrets
		SET status = 'revoked', revoked_at = NOW(), updated_at = NOW()
		WHERE tenant_id = $1 AND secret_key = $2 AND status <> 'revoked'
	`, tenantID, secretKey)
	if err != nil {
		return false, fmt.Errorf("delete tenant secret: %w", err)
	}
	return commandTag.RowsAffected() > 0, nil
}

func (s *Store) ListTemplates(ctx context.Context) ([]domain.Template, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id::text, code, name, description,
		       base_image_policy::text, default_channels::text, default_skills::text,
		       default_files::text, resource_policy::text, enabled
		FROM deployment_templates
		ORDER BY code ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("list templates: %w", err)
	}
	defer rows.Close()

	var templates []domain.Template
	for rows.Next() {
		template, err := scanTemplate(rows)
		if err != nil {
			return nil, err
		}
		templates = append(templates, template)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list templates rows: %w", err)
	}
	return templates, nil
}

func (s *Store) GetTemplate(ctx context.Context, templateID string) (domain.Template, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id::text, code, name, description,
		       base_image_policy::text, default_channels::text, default_skills::text,
		       default_files::text, resource_policy::text, enabled
		FROM deployment_templates
		WHERE id = $1
	`, templateID)
	return scanTemplate(row)
}

func (s *Store) ListImages(ctx context.Context, status string, imageRef string) ([]domain.Image, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id::text, image_ref, version, runtime_family, status, default_template_id::text, created_at, updated_at
		FROM image_catalog
		WHERE ($1 = '' OR status = $1)
		  AND ($2 = '' OR image_ref = $2)
		ORDER BY image_ref ASC, version DESC
	`, strings.TrimSpace(status), strings.TrimSpace(imageRef))
	if err != nil {
		return nil, fmt.Errorf("list images: %w", err)
	}
	defer rows.Close()

	var images []domain.Image
	for rows.Next() {
		image, err := scanImage(rows)
		if err != nil {
			return nil, err
		}
		images = append(images, image)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list images rows: %w", err)
	}
	return images, nil
}

func (s *Store) ResolveImageForTemplate(ctx context.Context, templateID string) (domain.Image, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id::text, image_ref, version, runtime_family, status, default_template_id::text, created_at, updated_at
		FROM image_catalog
		WHERE status = 'active' AND (default_template_id::text = $1 OR $1 = '')
		ORDER BY updated_at DESC
		LIMIT 1
	`, templateID)
	image, err := scanImage(row)
	if err == nil {
		return image, nil
	}
	if err != pgx.ErrNoRows {
		return domain.Image{}, err
	}
	row = s.pool.QueryRow(ctx, `
		SELECT id::text, image_ref, version, runtime_family, status, default_template_id::text, created_at, updated_at
		FROM image_catalog
		WHERE status = 'active'
		ORDER BY updated_at DESC
		LIMIT 1
	`)
	return scanImage(row)
}

func (s *Store) ListActiveTenantSecretsValues(ctx context.Context, tenantID string) ([]domain.DecryptedSecret, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT secret_key, secret_type, pgp_sym_decrypt(encrypted_value, $2)::text
		FROM tenant_secrets
		WHERE tenant_id = $1 AND status = 'active'
		ORDER BY secret_key ASC
	`, tenantID, s.masterKey)
	if err != nil {
		return nil, fmt.Errorf("list active tenant secrets values: %w", err)
	}
	defer rows.Close()

	var secrets []domain.DecryptedSecret
	for rows.Next() {
		var secret domain.DecryptedSecret
		if err := rows.Scan(&secret.SecretKey, &secret.SecretType, &secret.Value); err != nil {
			return nil, fmt.Errorf("scan decrypted secret: %w", err)
		}
		secrets = append(secrets, secret)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list active tenant secrets values rows: %w", err)
	}
	return secrets, nil
}

func (s *Store) UpdateTenantStatus(ctx context.Context, tenantID string, status string) error {
	_, err := s.pool.Exec(ctx, `UPDATE tenants SET status = $2, updated_at = NOW() WHERE id = $1`, tenantID, status)
	if err != nil {
		return fmt.Errorf("update tenant status: %w", err)
	}
	return nil
}

func (s *Store) UpsertLLMProvider(ctx context.Context, input domain.UpsertLLMProviderRequest) (domain.LLMProvider, error) {
	if input.Status == "" {
		input.Status = "active"
	}
	row := s.pool.QueryRow(ctx, `
		INSERT INTO llm_providers (name, display_name, description, base_url, status)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (name) DO UPDATE SET
			display_name = EXCLUDED.display_name,
			description = EXCLUDED.description,
			base_url = EXCLUDED.base_url,
			status = EXCLUDED.status,
			updated_at = NOW()
		RETURNING id::text, name, display_name, description, base_url, status, created_at, updated_at
	`, strings.TrimSpace(input.Name), strings.TrimSpace(input.DisplayName), strings.TrimSpace(input.Description), strings.TrimSpace(input.BaseURL), input.Status)
	return scanLLMProvider(row)
}

func (s *Store) ListLLMProviders(ctx context.Context) ([]domain.LLMProvider, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id::text, name, display_name, description, base_url, status, created_at, updated_at
		FROM llm_providers
		ORDER BY name ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("list llm providers: %w", err)
	}
	defer rows.Close()
	var providers []domain.LLMProvider
	for rows.Next() {
		p, err := scanLLMProvider(rows)
		if err != nil {
			return nil, err
		}
		providers = append(providers, p)
	}
	return providers, rows.Err()
}

func (s *Store) GetLLMProvider(ctx context.Context, providerID string) (domain.LLMProvider, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id::text, name, display_name, description, base_url, status, created_at, updated_at
		FROM llm_providers
		WHERE id = $1
	`, providerID)
	return scanLLMProvider(row)
}

func (s *Store) DeleteLLMProvider(ctx context.Context, providerID string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM llm_providers WHERE id = $1`, providerID)
	if err != nil {
		return fmt.Errorf("delete llm provider: %w", err)
	}
	return nil
}

func (s *Store) AddLLMAPIKey(ctx context.Context, providerID string, rawKey string, status string) (domain.LLMAPIKey, error) {
	if status == "" {
		status = "active"
	}
	fingerprint := keyFingerprint(rawKey)
	providerName, err := s.getProviderName(ctx, providerID)
	if err != nil {
		return domain.LLMAPIKey{}, err
	}
	row := s.pool.QueryRow(ctx, `
		INSERT INTO llm_api_keys (provider_id, encrypted_value, key_fingerprint, allocated_count, status, provider_name)
		VALUES ($1, pgp_sym_encrypt($2, $3), $4, 0, $5, $6)
		RETURNING id::text, provider_id::text, provider_name, key_fingerprint, allocated_count, status, created_at, updated_at
	`, providerID, rawKey, s.masterKey, fingerprint, status, providerName)
	key, err := scanLLMAPIKey(row)
	if err != nil {
		return domain.LLMAPIKey{}, err
	}
	return key, nil
}

func (s *Store) ListLLMAPIKeys(ctx context.Context, providerID string) ([]domain.LLMAPIKey, error) {
	query := `
		SELECT id::text, provider_id::text, provider_name, key_fingerprint, allocated_count, status, created_at, updated_at
		FROM llm_api_keys
	`
	args := []any{}
	if providerID != "" {
		query += ` WHERE provider_id = $1`
		args = append(args, providerID)
	}
	query += ` ORDER BY created_at DESC`
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list llm api keys: %w", err)
	}
	defer rows.Close()
	var keys []domain.LLMAPIKey
	for rows.Next() {
		k, err := scanLLMAPIKey(rows)
		if err != nil {
			return nil, err
		}
		keys = append(keys, k)
	}
	return keys, rows.Err()
}

func (s *Store) GetLLMAPIKey(ctx context.Context, keyID string) (domain.LLMAPIKey, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id::text, provider_id::text, provider_name, key_fingerprint, allocated_count, status, created_at, updated_at
		FROM llm_api_keys
		WHERE id = $1
	`, keyID)
	return scanLLMAPIKey(row)
}

func (s *Store) DecryptLLMAPIKey(ctx context.Context, keyID string) (string, error) {
	var raw string
	err := s.pool.QueryRow(ctx, `
		SELECT pgp_sym_decrypt(encrypted_value, $2)::text
		FROM llm_api_keys
		WHERE id = $1 AND status = 'active'
	`, keyID, s.masterKey).Scan(&raw)
	if err != nil {
		return "", fmt.Errorf("decrypt llm api key: %w", err)
	}
	return raw, nil
}

func (s *Store) DeactivateLLMAPIKey(ctx context.Context, keyID string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE llm_api_keys SET status = 'inactive', updated_at = NOW() WHERE id = $1
	`, keyID)
	if err != nil {
		return fmt.Errorf("deactivate llm api key: %w", err)
	}
	return nil
}

func (s *Store) ListActiveLLMAPIKeysByProvider(ctx context.Context, providerName string) ([]domain.LLMAPIKey, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id::text, provider_id::text, provider_name, key_fingerprint, allocated_count, status, created_at, updated_at
		FROM llm_api_keys
		WHERE provider_name = $1 AND status = 'active'
		ORDER BY allocated_count ASC, created_at ASC
	`, providerName)
	if err != nil {
		return nil, fmt.Errorf("list active llm api keys by provider: %w", err)
	}
	defer rows.Close()
	var keys []domain.LLMAPIKey
	for rows.Next() {
		k, err := scanLLMAPIKey(rows)
		if err != nil {
			return nil, err
		}
		keys = append(keys, k)
	}
	return keys, rows.Err()
}

func (s *Store) AllocateLLMKeyToTenant(ctx context.Context, tenantID string, apiKeyID string, modelName string) (domain.TenantLLMKeyAllocation, error) {
	var providerID string
	err := s.pool.QueryRow(ctx, `SELECT provider_id::text FROM llm_api_keys WHERE id = $1`, apiKeyID).Scan(&providerID)
	if err != nil {
		return domain.TenantLLMKeyAllocation{}, fmt.Errorf("get provider for api key: %w", err)
	}

	row := s.pool.QueryRow(ctx, `
		INSERT INTO tenant_llm_key_allocations (tenant_id, provider_id, api_key_id, model_name)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (tenant_id, provider_id) DO UPDATE SET
			api_key_id = EXCLUDED.api_key_id,
			model_name = EXCLUDED.model_name
		RETURNING id::text, tenant_id::text, provider_id::text, api_key_id::text, model_name, created_at
	`, tenantID, providerID, apiKeyID, modelName)
	alloc, err := scanTenantLLMAllocation(row)
	if err != nil {
		return domain.TenantLLMKeyAllocation{}, err
	}

	_, err = s.pool.Exec(ctx, `
		UPDATE llm_api_keys SET allocated_count = allocated_count + 1, updated_at = NOW() WHERE id = $1
	`, apiKeyID)
	if err != nil {
		return domain.TenantLLMKeyAllocation{}, fmt.Errorf("increment allocated count: %w", err)
	}

	return alloc, nil
}

func (s *Store) GetTenantLLMAllocation(ctx context.Context, tenantID string) (domain.TenantLLMKeyAllocation, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id::text, tenant_id::text, provider_id::text, api_key_id::text, model_name, created_at
		FROM tenant_llm_key_allocations
		WHERE tenant_id = $1
	`, tenantID)
	return scanTenantLLMAllocation(row)
}

func (s *Store) RevokeTenantLLMAllocation(ctx context.Context, tenantID string) error {
	var apiKeyID string
	err := s.pool.QueryRow(ctx, `
		SELECT api_key_id::text FROM tenant_llm_key_allocations WHERE tenant_id = $1
	`, tenantID).Scan(&apiKeyID)
	if err != nil {
		return fmt.Errorf("find allocation to revoke: %w", err)
	}

	_, err = s.pool.Exec(ctx, `DELETE FROM tenant_llm_key_allocations WHERE tenant_id = $1`, tenantID)
	if err != nil {
		return fmt.Errorf("revoke llm allocation: %w", err)
	}

	_, err = s.pool.Exec(ctx, `
		UPDATE llm_api_keys SET allocated_count = GREATEST(allocated_count - 1, 0), updated_at = NOW()
		WHERE id = $1
	`, apiKeyID)
	if err != nil {
		return fmt.Errorf("decrement allocated count: %w", err)
	}
	return nil
}

func (s *Store) getProviderName(ctx context.Context, providerID string) (string, error) {
	var name string
	err := s.pool.QueryRow(ctx, `SELECT name FROM llm_providers WHERE id = $1`, providerID).Scan(&name)
	return name, err
}

func keyFingerprint(rawKey string) string {
	if len(rawKey) <= 8 {
		return rawKey[:2] + "****" + rawKey[len(rawKey)-2:]
	}
	return rawKey[:4] + "****" + rawKey[len(rawKey)-4:]
}

func (s *Store) FindActiveJob(ctx context.Context, tenantID string) (domain.DeploymentJob, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id::text, tenant_id::text, job_type, requested_by, COALESCE(idempotency_key, ''), status,
		       payload::text, attempt_count, COALESCE(last_error, ''), scheduled_at, started_at, finished_at,
		       COALESCE(worker_name, ''), heartbeat_at
		FROM deployment_jobs
		WHERE tenant_id = $1 AND status IN ('pending', 'running')
		ORDER BY scheduled_at ASC
		LIMIT 1
	`, tenantID)
	return scanDeploymentJob(row)
}

func (s *Store) EnqueueDeploymentJob(ctx context.Context, tenantID string, jobType string, requestedBy string, idempotencyKey string, payload json.RawMessage) (domain.DeploymentJob, error) {
	normalizedPayload, err := normalizeJSON(payload, `{}`)
	if err != nil {
		return domain.DeploymentJob{}, err
	}
	if strings.TrimSpace(idempotencyKey) != "" {
		row := s.pool.QueryRow(ctx, `
			SELECT id::text, tenant_id::text, job_type, requested_by, COALESCE(idempotency_key, ''), status,
			       payload::text, attempt_count, COALESCE(last_error, ''), scheduled_at, started_at, finished_at,
			       COALESCE(worker_name, ''), heartbeat_at
			FROM deployment_jobs
			WHERE tenant_id = $1 AND job_type = $2 AND idempotency_key = $3
			ORDER BY scheduled_at DESC
			LIMIT 1
		`, tenantID, jobType, idempotencyKey)
		job, scanErr := scanDeploymentJob(row)
		if scanErr == nil {
			return job, nil
		}
		if scanErr != pgx.ErrNoRows {
			return domain.DeploymentJob{}, scanErr
		}
	}

	row := s.pool.QueryRow(ctx, `
		INSERT INTO deployment_jobs (tenant_id, job_type, requested_by, idempotency_key, status, payload, scheduled_at)
		VALUES ($1, $2, $3, NULLIF($4, ''), 'pending', $5::jsonb, NOW())
		RETURNING id::text, tenant_id::text, job_type, requested_by, COALESCE(idempotency_key, ''), status,
		          payload::text, attempt_count, COALESCE(last_error, ''), scheduled_at, started_at, finished_at,
		          COALESCE(worker_name, ''), heartbeat_at
	`, tenantID, jobType, requestedBy, idempotencyKey, string(normalizedPayload))
	return scanDeploymentJob(row)
}

func (s *Store) GetDeploymentJob(ctx context.Context, jobID string) (domain.DeploymentJob, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id::text, tenant_id::text, job_type, requested_by, COALESCE(idempotency_key, ''), status,
		       payload::text, attempt_count, COALESCE(last_error, ''), scheduled_at, started_at, finished_at,
		       COALESCE(worker_name, ''), heartbeat_at
		FROM deployment_jobs
		WHERE id = $1
	`, jobID)
	return scanDeploymentJob(row)
}

func (s *Store) ListDeploymentJobs(ctx context.Context, filter domain.DeploymentJobFilter) ([]domain.DeploymentJob, error) {
	page := filter.Page
	if page < 1 {
		page = 1
	}
	pageSize := filter.PageSize
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	offset := (page - 1) * pageSize

	rows, err := s.pool.Query(ctx, `
		SELECT id::text, tenant_id::text, job_type, requested_by, COALESCE(idempotency_key, ''), status,
		       payload::text, attempt_count, COALESCE(last_error, ''), scheduled_at, started_at, finished_at,
		       COALESCE(worker_name, ''), heartbeat_at
		FROM deployment_jobs
		WHERE ($1 = '' OR tenant_id::text = $1)
		  AND ($2 = '' OR job_type = $2)
		  AND ($3 = '' OR status = $3)
		ORDER BY scheduled_at DESC
		LIMIT $4 OFFSET $5
	`, filter.TenantID, strings.TrimSpace(filter.JobType), strings.TrimSpace(filter.Status), pageSize, offset)
	if err != nil {
		return nil, fmt.Errorf("list deployment jobs: %w", err)
	}
	defer rows.Close()

	var jobs []domain.DeploymentJob
	for rows.Next() {
		job, err := scanDeploymentJob(rows)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list deployment jobs rows: %w", err)
	}
	return jobs, nil
}

func (s *Store) ClaimNextPendingJob(ctx context.Context, workerName string, jobHeartbeatTTL time.Duration) (domain.DeploymentJob, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return domain.DeploymentJob{}, fmt.Errorf("begin claim job tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var jobID string
	abandonedThreshold := time.Now().Add(-jobHeartbeatTTL)

	err = tx.QueryRow(ctx, `
		SELECT id::text
		FROM deployment_jobs
		WHERE
			(status = 'pending')
			OR (status = 'running' AND heartbeat_at IS NOT NULL AND heartbeat_at < $1::timestamptz)
		ORDER BY
			CASE WHEN status = 'pending' THEN 0 ELSE 1 END,
			scheduled_at ASC
		FOR UPDATE SKIP LOCKED
		LIMIT 1
	`, abandonedThreshold).Scan(&jobID)
	if err != nil {
		return domain.DeploymentJob{}, err
	}

	row := tx.QueryRow(ctx, `
		UPDATE deployment_jobs
		SET
			status = 'running',
			worker_name = $2,
			attempt_count = CASE WHEN status = 'pending' THEN attempt_count + 1 ELSE attempt_count + 0 END,
			started_at = CASE WHEN status = 'pending' THEN NOW() ELSE started_at END,
			heartbeat_at = NOW(),
			finished_at = NULL
		WHERE id = $1
		RETURNING id::text, tenant_id::text, job_type, requested_by, COALESCE(idempotency_key, ''), status,
		          payload::text, attempt_count, COALESCE(last_error, ''), scheduled_at, started_at, finished_at,
		          COALESCE(worker_name, ''), heartbeat_at
	`, jobID, workerName)
	job, err := scanDeploymentJob(row)
	if err != nil {
		return domain.DeploymentJob{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.DeploymentJob{}, fmt.Errorf("commit claim job tx: %w", err)
	}
	return job, nil
}

func (s *Store) MarkJobSucceeded(ctx context.Context, jobID string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE deployment_jobs
		SET status = 'succeeded', last_error = NULL, finished_at = NOW(), heartbeat_at = NULL, worker_name = NULL
		WHERE id = $1
	`, jobID)
	if err != nil {
		return fmt.Errorf("mark job succeeded: %w", err)
	}
	return nil
}

func (s *Store) MarkJobFailed(ctx context.Context, jobID string, lastError string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE deployment_jobs
		SET status = 'failed', last_error = $2, finished_at = NOW(), heartbeat_at = NULL, worker_name = NULL
		WHERE id = $1
	`, jobID, lastError)
	if err != nil {
		return fmt.Errorf("mark job failed: %w", err)
	}
	return nil
}

func (s *Store) UpdateJobHeartbeat(ctx context.Context, jobID string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE deployment_jobs SET heartbeat_at = NOW() WHERE id = $1
	`, jobID)
	if err != nil {
		return fmt.Errorf("update job heartbeat: %w", err)
	}
	return nil
}

func (s *Store) NextConfigVersion(ctx context.Context, tenantID string) (int, error) {
	var version int
	err := s.pool.QueryRow(ctx, `SELECT COALESCE(MAX(config_version), 0) + 1 FROM tenant_instances WHERE tenant_id = $1`, tenantID).Scan(&version)
	if err != nil {
		return 0, fmt.Errorf("next config version: %w", err)
	}
	return version, nil
}

func (s *Store) CreateTenantInstance(ctx context.Context, params CreateTenantInstanceParams) (domain.TenantInstance, error) {
	status := params.Status
	if strings.TrimSpace(status) == "" {
		status = "creating"
	}
	health := params.HealthStatus
	if strings.TrimSpace(health) == "" {
		health = "unknown"
	}
	row := s.pool.QueryRow(ctx, `
		INSERT INTO tenant_instances (
			tenant_id, deployment_job_id, container_name, image_ref, status, host_node,
			workspace_path, volume_name, route_url, health_status, config_version, created_at, updated_at
		)
		VALUES (
			$1, NULLIF($2, '')::uuid, $3, $4, $5, $6,
			$7, $8, $9, $10, $11, NOW(), NOW()
		)
		RETURNING id::text, tenant_id::text, COALESCE(deployment_job_id::text, ''), COALESCE(container_id, ''),
		          COALESCE(container_name, ''), COALESCE(image_ref, ''), status, COALESCE(host_node, ''),
		          COALESCE(workspace_path, ''), COALESCE(volume_name, ''), COALESCE(route_url, ''), health_status,
		          config_version, started_at, stopped_at, created_at, updated_at
	`, params.TenantID, params.DeploymentJobID, params.ContainerName, params.ImageRef, status, params.HostNode,
		params.WorkspacePath, params.VolumeName, params.RouteURL, health, params.ConfigVersion)
	return scanTenantInstance(row)
}

func (s *Store) GetCurrentTenantInstance(ctx context.Context, tenantID string) (domain.TenantInstance, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id::text, tenant_id::text, COALESCE(deployment_job_id::text, ''), COALESCE(container_id, ''),
		       COALESCE(container_name, ''), COALESCE(image_ref, ''), status, COALESCE(host_node, ''),
		       COALESCE(workspace_path, ''), COALESCE(volume_name, ''), COALESCE(route_url, ''), health_status,
		       config_version, started_at, stopped_at, created_at, updated_at
		FROM tenant_instances
		WHERE tenant_id = $1
		ORDER BY created_at DESC
		LIMIT 1
	`, tenantID)
	return scanTenantInstance(row)
}

func (s *Store) ListTenantInstances(ctx context.Context, tenantID string) ([]domain.TenantInstance, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id::text, tenant_id::text, COALESCE(deployment_job_id::text, ''), COALESCE(container_id, ''),
		       COALESCE(container_name, ''), COALESCE(image_ref, ''), status, COALESCE(host_node, ''),
		       COALESCE(workspace_path, ''), COALESCE(volume_name, ''), COALESCE(route_url, ''), health_status,
		       config_version, started_at, stopped_at, created_at, updated_at
		FROM tenant_instances
		WHERE tenant_id = $1
		ORDER BY created_at DESC
	`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("list tenant instances: %w", err)
	}
	defer rows.Close()

	var instances []domain.TenantInstance
	for rows.Next() {
		instance, err := scanTenantInstance(rows)
		if err != nil {
			return nil, err
		}
		instances = append(instances, instance)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list tenant instances rows: %w", err)
	}
	return instances, nil
}

func (s *Store) UpdateInstanceRunning(ctx context.Context, instanceID string, containerID string, containerName string, healthStatus string) (domain.TenantInstance, error) {
	row := s.pool.QueryRow(ctx, `
		UPDATE tenant_instances
		SET container_id = $2, container_name = $3, status = 'running', health_status = $4,
		    started_at = NOW(), stopped_at = NULL, updated_at = NOW()
		WHERE id = $1
		RETURNING id::text, tenant_id::text, COALESCE(deployment_job_id::text, ''), COALESCE(container_id, ''),
		          COALESCE(container_name, ''), COALESCE(image_ref, ''), status, COALESCE(host_node, ''),
		          COALESCE(workspace_path, ''), COALESCE(volume_name, ''), COALESCE(route_url, ''), health_status,
		          config_version, started_at, stopped_at, created_at, updated_at
	`, instanceID, containerID, containerName, healthStatus)
	return scanTenantInstance(row)
}

func (s *Store) UpdateInstanceStatus(ctx context.Context, instanceID string, status string, healthStatus string) (domain.TenantInstance, error) {
	row := s.pool.QueryRow(ctx, `
		UPDATE tenant_instances
		SET status = $2,
		    health_status = $3,
		    stopped_at = CASE WHEN $2 IN ('stopped', 'destroyed', 'failed') THEN NOW() ELSE stopped_at END,
		    started_at = CASE WHEN $2 = 'running' AND started_at IS NULL THEN NOW() ELSE started_at END,
		    updated_at = NOW()
		WHERE id = $1
		RETURNING id::text, tenant_id::text, COALESCE(deployment_job_id::text, ''), COALESCE(container_id, ''),
		          COALESCE(container_name, ''), COALESCE(image_ref, ''), status, COALESCE(host_node, ''),
		          COALESCE(workspace_path, ''), COALESCE(volume_name, ''), COALESCE(route_url, ''), health_status,
		          config_version, started_at, stopped_at, created_at, updated_at
	`, instanceID, status, healthStatus)
	return scanTenantInstance(row)
}

func (s *Store) UpsertWorkerHeartbeat(ctx context.Context, workerName string) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO worker_heartbeats (worker_name, last_seen_at)
		VALUES ($1, NOW())
		ON CONFLICT (worker_name) DO UPDATE SET last_seen_at = NOW()
	`, workerName)
	if err != nil {
		return fmt.Errorf("upsert worker heartbeat: %w", err)
	}
	return nil
}

func (s *Store) GetWorkerHeartbeat(ctx context.Context, workerName string) (time.Time, error) {
	var lastSeen time.Time
	err := s.pool.QueryRow(ctx, `SELECT last_seen_at FROM worker_heartbeats WHERE worker_name = $1`, workerName).Scan(&lastSeen)
	if err != nil {
		return time.Time{}, err
	}
	return lastSeen, nil
}

type scanner interface {
	Scan(dest ...any) error
}

func scanTenant(row scanner) (domain.Tenant, error) {
	var tenant domain.Tenant
	if err := row.Scan(&tenant.ID, &tenant.ExternalUserID, &tenant.Slug, &tenant.DisplayName, &tenant.Status, &tenant.CreatedAt, &tenant.UpdatedAt); err != nil {
		return domain.Tenant{}, err
	}
	return tenant, nil
}

func scanProfile(row scanner) (domain.TenantProfile, error) {
	var profile domain.TenantProfile
	var channels string
	var skills string
	var extraFiles string
	var validationErrors string
	if err := row.Scan(
		&profile.TenantID,
		&profile.TemplateID,
		&profile.ResourceTier,
		&profile.RouteKey,
		&profile.ModelProvider,
		&profile.ModelName,
		&channels,
		&skills,
		&profile.SoulMarkdown,
		&profile.MemoryMarkdown,
		&extraFiles,
		&profile.Validation.IsValid,
		&validationErrors,
		&profile.UpdatedAt,
	); err != nil {
		return domain.TenantProfile{}, err
	}
	profile.Channels = json.RawMessage(channels)
	profile.Skills = json.RawMessage(skills)
	profile.ExtraFiles = json.RawMessage(extraFiles)
	if validationErrors != "" {
		if err := json.Unmarshal([]byte(validationErrors), &profile.Validation.Errors); err != nil {
			return domain.TenantProfile{}, fmt.Errorf("decode validation errors: %w", err)
		}
	}
	if profile.Validation.Errors == nil {
		profile.Validation.Errors = []domain.ValidationIssue{}
	}
	return profile, nil
}

func scanSecretMetadata(row scanner) (domain.SecretMetadata, error) {
	var secret domain.SecretMetadata
	var revokedAt sql.NullTime
	if err := row.Scan(
		&secret.TenantID,
		&secret.SecretKey,
		&secret.SecretType,
		&secret.ValueFingerprint,
		&secret.Version,
		&secret.Status,
		&secret.CreatedAt,
		&secret.UpdatedAt,
		&revokedAt,
	); err != nil {
		return domain.SecretMetadata{}, err
	}
	if revokedAt.Valid {
		secret.RevokedAt = revokedAt.Time
		secret.HasRevokedAtValue = true
	}
	return secret, nil
}

func scanTemplate(row scanner) (domain.Template, error) {
	var template domain.Template
	var baseImagePolicy string
	var defaultChannels string
	var defaultSkills string
	var defaultFiles string
	var resourcePolicy string
	if err := row.Scan(
		&template.ID,
		&template.Code,
		&template.Name,
		&template.Description,
		&baseImagePolicy,
		&defaultChannels,
		&defaultSkills,
		&defaultFiles,
		&resourcePolicy,
		&template.Enabled,
	); err != nil {
		return domain.Template{}, err
	}
	template.BaseImagePolicy = json.RawMessage(baseImagePolicy)
	template.DefaultChannels = json.RawMessage(defaultChannels)
	template.DefaultSkills = json.RawMessage(defaultSkills)
	template.DefaultFiles = json.RawMessage(defaultFiles)
	template.ResourcePolicy = json.RawMessage(resourcePolicy)
	return template, nil
}

func scanLLMProvider(row scanner) (domain.LLMProvider, error) {
	var p domain.LLMProvider
	if err := row.Scan(&p.ID, &p.Name, &p.DisplayName, &p.Description, &p.BaseURL, &p.Status, &p.CreatedAt, &p.UpdatedAt); err != nil {
		return domain.LLMProvider{}, err
	}
	return p, nil
}

func scanLLMAPIKey(row scanner) (domain.LLMAPIKey, error) {
	var k domain.LLMAPIKey
	if err := row.Scan(&k.ID, &k.ProviderID, &k.ProviderName, &k.KeyFingerprint, &k.AllocatedCount, &k.Status, &k.CreatedAt, &k.UpdatedAt); err != nil {
		return domain.LLMAPIKey{}, err
	}
	return k, nil
}

func scanTenantLLMAllocation(row scanner) (domain.TenantLLMKeyAllocation, error) {
	var a domain.TenantLLMKeyAllocation
	if err := row.Scan(&a.ID, &a.TenantID, &a.ProviderID, &a.APIKeyID, &a.ModelName, &a.CreatedAt); err != nil {
		return domain.TenantLLMKeyAllocation{}, err
	}
	return a, nil
}

func scanImage(row scanner) (domain.Image, error) {
	var image domain.Image
	if err := row.Scan(&image.ID, &image.ImageRef, &image.Version, &image.RuntimeFamily, &image.Status, &image.DefaultTemplateID, &image.CreatedAt, &image.UpdatedAt); err != nil {
		return domain.Image{}, err
	}
	return image, nil
}

func scanDeploymentJob(row scanner) (domain.DeploymentJob, error) {
	var job domain.DeploymentJob
	var payload string
	var startedAt sql.NullTime
	var finishedAt sql.NullTime
	var heartbeatAt sql.NullTime
	if err := row.Scan(&job.ID, &job.TenantID, &job.JobType, &job.RequestedBy, &job.IdempotencyKey, &job.Status, &payload, &job.AttemptCount, &job.LastError, &job.ScheduledAt, &startedAt, &finishedAt, &job.WorkerName, &heartbeatAt); err != nil {
		return domain.DeploymentJob{}, err
	}
	job.Payload = json.RawMessage(payload)
	if startedAt.Valid {
		started := startedAt.Time
		job.StartedAt = &started
	}
	if finishedAt.Valid {
		finished := finishedAt.Time
		job.FinishedAt = &finished
	}
	if heartbeatAt.Valid {
		hb := heartbeatAt.Time
		job.HeartbeatAt = &hb
	}
	return job, nil
}

func scanTenantInstance(row scanner) (domain.TenantInstance, error) {
	var instance domain.TenantInstance
	var startedAt sql.NullTime
	var stoppedAt sql.NullTime
	if err := row.Scan(
		&instance.ID,
		&instance.TenantID,
		&instance.DeploymentJobID,
		&instance.ContainerID,
		&instance.ContainerName,
		&instance.ImageRef,
		&instance.Status,
		&instance.HostNode,
		&instance.WorkspacePath,
		&instance.VolumeName,
		&instance.RouteURL,
		&instance.HealthStatus,
		&instance.ConfigVersion,
		&startedAt,
		&stoppedAt,
		&instance.CreatedAt,
		&instance.UpdatedAt,
	); err != nil {
		return domain.TenantInstance{}, err
	}
	if startedAt.Valid {
		started := startedAt.Time
		instance.StartedAt = &started
	}
	if stoppedAt.Valid {
		stopped := stoppedAt.Time
		instance.StoppedAt = &stopped
	}
	return instance, nil
}

func normalizeJSON(raw json.RawMessage, fallback string) (json.RawMessage, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return json.RawMessage(fallback), nil
	}
	if !json.Valid(trimmed) {
		return nil, fmt.Errorf("invalid json payload")
	}
	return append(json.RawMessage(nil), trimmed...), nil
}

func (s *Store) CountTenants(ctx context.Context) (int, error) {
	var count int
	err := s.pool.QueryRow(ctx, "SELECT COUNT(*) FROM tenants").Scan(&count)
	return count, err
}

func (s *Store) CountRunningContainers(ctx context.Context) (int, error) {
	var count int
	err := s.pool.QueryRow(ctx, "SELECT COUNT(*) FROM tenant_instances WHERE status = 'running'").Scan(&count)
	return count, err
}

func (s *Store) CountPendingJobs(ctx context.Context) (int, error) {
	var count int
	err := s.pool.QueryRow(ctx, "SELECT COUNT(*) FROM deployment_jobs WHERE status = 'pending'").Scan(&count)
	return count, err
}

func (s *Store) CountJobsByStatus(ctx context.Context, status string) (int, error) {
	var count int
	err := s.pool.QueryRow(ctx, "SELECT COUNT(*) FROM deployment_jobs WHERE status = $1", status).Scan(&count)
	return count, err
}
