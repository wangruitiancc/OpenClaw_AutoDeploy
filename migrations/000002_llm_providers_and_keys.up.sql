CREATE TABLE IF NOT EXISTS llm_providers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL UNIQUE,
    display_name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    base_url TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS llm_api_keys (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    provider_id UUID NOT NULL REFERENCES llm_providers(id) ON DELETE CASCADE,
    encrypted_value BYTEA NOT NULL,
    key_fingerprint TEXT NOT NULL,
    provider_name TEXT NOT NULL,
    allocated_count INTEGER NOT NULL DEFAULT 0,
    status TEXT NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS tenant_llm_key_allocations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    provider_id UUID NOT NULL REFERENCES llm_providers(id) ON DELETE CASCADE,
    api_key_id UUID NOT NULL REFERENCES llm_api_keys(id) ON DELETE CASCADE,
    model_name TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (tenant_id, provider_id)
);

CREATE INDEX IF NOT EXISTS idx_llm_api_keys_provider_id ON llm_api_keys(provider_id);
CREATE INDEX IF NOT EXISTS idx_llm_api_keys_status ON llm_api_keys(status);
CREATE INDEX IF NOT EXISTS idx_tenant_llm_allocations_tenant_id ON tenant_llm_key_allocations(tenant_id);
CREATE INDEX IF NOT EXISTS idx_tenant_llm_allocations_api_key_id ON tenant_llm_key_allocations(api_key_id);
