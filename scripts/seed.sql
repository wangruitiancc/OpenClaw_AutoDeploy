INSERT INTO deployment_templates (
    code,
    name,
    description,
    base_image_policy,
    default_channels,
    default_skills,
    default_files,
    resource_policy,
    enabled
)
VALUES (
    'local-smoke',
    'Local Smoke Template',
    'Default local template for smoke-testing the control plane with a long-running container image.',
    '{"selection":"default"}'::jsonb,
    '[]'::jsonb,
    '["smoke-test"]'::jsonb,
    '[]'::jsonb,
    '{"tier":"standard"}'::jsonb,
    TRUE
)
ON CONFLICT (code) DO UPDATE SET
    name = EXCLUDED.name,
    description = EXCLUDED.description,
    base_image_policy = EXCLUDED.base_image_policy,
    default_channels = EXCLUDED.default_channels,
    default_skills = EXCLUDED.default_skills,
    default_files = EXCLUDED.default_files,
    resource_policy = EXCLUDED.resource_policy,
    enabled = EXCLUDED.enabled,
    updated_at = NOW();

INSERT INTO image_catalog (
    image_ref,
    version,
    runtime_family,
    status,
    default_template_id
)
SELECT
    'ghcr.io/openclaw/openclaw:latest',
    'local-smoke',
    'smoke-test',
    'active',
    id
FROM deployment_templates
WHERE code = 'local-smoke'
ON CONFLICT (image_ref, version) DO UPDATE SET
    runtime_family = EXCLUDED.runtime_family,
    status = EXCLUDED.status,
    default_template_id = EXCLUDED.default_template_id,
    updated_at = NOW();
