package service

import (
	"context"
	"encoding/json"
	"strings"

	"openclaw-autodeploy/internal/domain"
)

const maxMarkdownBytes = 256 * 1024
const maxCombinedJSONBytes = 2 * 1024 * 1024

type ValidationStore interface {
	TemplateExists(ctx context.Context, templateID string) (bool, error)
	RouteKeyInUse(ctx context.Context, tenantID string, routeKey string) (bool, error)
	SecretExists(ctx context.Context, tenantID string, secretKey string) (bool, error)
}

type ProfileValidator struct {
	store ValidationStore
}

func NewProfileValidator(store ValidationStore) *ProfileValidator {
	return &ProfileValidator{store: store}
}

func (v *ProfileValidator) Validate(ctx context.Context, tenantID string, input domain.UpsertTenantProfileRequest) (domain.ValidationResult, error) {
	issues := make([]domain.ValidationIssue, 0)

	if strings.TrimSpace(input.TemplateID) == "" {
		issues = append(issues, issue("template_id", "template_id is required"))
	} else {
		exists, err := v.store.TemplateExists(ctx, input.TemplateID)
		if err != nil {
			return domain.ValidationResult{}, err
		}
		if !exists {
			issues = append(issues, issue("template_id", "template does not exist or is not enabled"))
		}
	}

	if !isAllowedTier(input.ResourceTier) {
		issues = append(issues, issue("resource_tier", "resource_tier must be one of starter, standard, pro, enterprise"))
	}

	if strings.TrimSpace(input.RouteKey) == "" {
		issues = append(issues, issue("route_key", "route_key is required"))
	} else {
		inUse, err := v.store.RouteKeyInUse(ctx, tenantID, input.RouteKey)
		if err != nil {
			return domain.ValidationResult{}, err
		}
		if inUse {
			issues = append(issues, issue("route_key", "route_key is already in use"))
		}
	}

	if strings.TrimSpace(input.ModelProvider) == "" {
		issues = append(issues, issue("model_provider", "model_provider is required"))
	}
	if strings.TrimSpace(input.ModelName) == "" {
		issues = append(issues, issue("model_name", "model_name is required"))
	}

	jsonBytes := len(input.Channels) + len(input.Skills) + len(input.ExtraFiles)
	if jsonBytes > maxCombinedJSONBytes {
		issues = append(issues, issue("payload", "combined JSON payload exceeds size limit"))
	}

	if !isValidOptionalJSON(input.Channels) {
		issues = append(issues, issue("channels", "channels must be valid JSON"))
	}
	if !isValidOptionalJSON(input.Skills) {
		issues = append(issues, issue("skills", "skills must be valid JSON"))
	}
	if !isValidOptionalJSON(input.ExtraFiles) {
		issues = append(issues, issue("extra_files", "extra_files must be valid JSON"))
	}

	if len(input.SoulMarkdown) > maxMarkdownBytes {
		issues = append(issues, issue("soul_markdown", "SOUL.md exceeds 256KB"))
	}
	if len(input.MemoryMarkdown) > maxMarkdownBytes {
		issues = append(issues, issue("memory_markdown", "memory.md exceeds 256KB"))
	}

	for _, secretKey := range requiredSecrets(input.ModelProvider) {
		exists, err := v.store.SecretExists(ctx, tenantID, secretKey)
		if err != nil {
			return domain.ValidationResult{}, err
		}
		if !exists {
			issues = append(issues, issue("tenant_secrets."+secretKey, "missing required secret"))
		}
	}

	return domain.ValidationResult{
		IsValid: len(issues) == 0,
		Errors:  issues,
	}, nil
}

func issue(field string, message string) domain.ValidationIssue {
	return domain.ValidationIssue{Field: field, Message: message}
}

func isAllowedTier(value string) bool {
	switch strings.TrimSpace(value) {
	case "starter", "standard", "pro", "enterprise":
		return true
	default:
		return false
	}
}

func isValidOptionalJSON(raw json.RawMessage) bool {
	if len(strings.TrimSpace(string(raw))) == 0 {
		return true
	}
	return json.Valid(raw)
}

func requiredSecrets(provider string) []string {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "openai", "openai-compatible":
		return []string{"OPENAI_API_KEY"}
	case "anthropic":
		return []string{"ANTHROPIC_API_KEY"}
	default:
		return nil
	}
}
