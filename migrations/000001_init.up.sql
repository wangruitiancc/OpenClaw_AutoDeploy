CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS tenants (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    external_user_id TEXT NOT NULL UNIQUE,
    slug TEXT NOT NULL UNIQUE,
    display_name TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'draft',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS deployment_templates (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    base_image_policy JSONB NOT NULL DEFAULT '{}'::jsonb,
    default_channels JSONB NOT NULL DEFAULT '[]'::jsonb,
    default_skills JSONB NOT NULL DEFAULT '[]'::jsonb,
    default_files JSONB NOT NULL DEFAULT '[]'::jsonb,
    resource_policy JSONB NOT NULL DEFAULT '{}'::jsonb,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS image_catalog (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    image_ref TEXT NOT NULL,
    version TEXT NOT NULL,
    runtime_family TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'active',
    default_template_id UUID REFERENCES deployment_templates(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (image_ref, version)
);

CREATE TABLE IF NOT EXISTS tenant_profiles (
    tenant_id UUID PRIMARY KEY REFERENCES tenants(id) ON DELETE CASCADE,
    template_id UUID NOT NULL REFERENCES deployment_templates(id),
    resource_tier TEXT NOT NULL,
    route_key TEXT NOT NULL UNIQUE,
    model_provider TEXT NOT NULL,
    model_name TEXT NOT NULL,
    channels JSONB NOT NULL DEFAULT '[]'::jsonb,
    skills JSONB NOT NULL DEFAULT '[]'::jsonb,
    soul_markdown TEXT NOT NULL DEFAULT '',
    memory_markdown TEXT NOT NULL DEFAULT '',
    extra_files JSONB NOT NULL DEFAULT '[]'::jsonb,
    is_valid BOOLEAN NOT NULL DEFAULT FALSE,
    validation_errors JSONB NOT NULL DEFAULT '[]'::jsonb,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS tenant_secrets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    secret_type TEXT NOT NULL,
    secret_key TEXT NOT NULL,
    encrypted_value BYTEA NOT NULL,
    value_fingerprint TEXT NOT NULL,
    version INTEGER NOT NULL DEFAULT 1,
    status TEXT NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    revoked_at TIMESTAMPTZ,
    UNIQUE (tenant_id, secret_key)
);

CREATE TABLE IF NOT EXISTS deployment_jobs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    job_type TEXT NOT NULL,
    requested_by TEXT NOT NULL,
    idempotency_key TEXT,
    status TEXT NOT NULL DEFAULT 'pending',
    payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    attempt_count INTEGER NOT NULL DEFAULT 0,
    last_error TEXT,
    scheduled_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    started_at TIMESTAMPTZ,
    finished_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS tenant_instances (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    deployment_job_id UUID REFERENCES deployment_jobs(id),
    container_id TEXT,
    container_name TEXT,
    image_ref TEXT,
    status TEXT NOT NULL DEFAULT 'creating',
    host_node TEXT,
    workspace_path TEXT,
    volume_name TEXT,
    route_url TEXT,
    health_status TEXT NOT NULL DEFAULT 'unknown',
    config_version INTEGER NOT NULL DEFAULT 1,
    started_at TIMESTAMPTZ,
    stopped_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS audit_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID REFERENCES tenants(id) ON DELETE SET NULL,
    actor TEXT NOT NULL,
    action TEXT NOT NULL,
    target_type TEXT NOT NULL,
    target_id TEXT NOT NULL,
    request_id TEXT,
    details JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS worker_heartbeats (
    worker_name TEXT PRIMARY KEY,
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_tenants_status ON tenants(status);
CREATE INDEX IF NOT EXISTS idx_tenant_secrets_tenant_id ON tenant_secrets(tenant_id);
CREATE INDEX IF NOT EXISTS idx_deployment_jobs_tenant_id_status ON deployment_jobs(tenant_id, status);
CREATE INDEX IF NOT EXISTS idx_tenant_instances_tenant_id_status ON tenant_instances(tenant_id, status);
CREATE INDEX IF NOT EXISTS idx_audit_logs_tenant_id_created_at ON audit_logs(tenant_id, created_at DESC);
