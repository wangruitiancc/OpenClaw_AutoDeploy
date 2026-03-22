package service

import (
	"context"
	"encoding/json"
	"testing"

	"openclaw-autodeploy/internal/domain"
)

type fakeValidationStore struct {
	templateExists bool
	routeKeyInUse  bool
	secrets        map[string]bool
}

func (f fakeValidationStore) TemplateExists(context.Context, string) (bool, error) {
	return f.templateExists, nil
}

func (f fakeValidationStore) RouteKeyInUse(context.Context, string, string) (bool, error) {
	return f.routeKeyInUse, nil
}

func (f fakeValidationStore) SecretExists(_ context.Context, _ string, secretKey string) (bool, error) {
	return f.secrets[secretKey], nil
}

func TestValidateHappyPath(t *testing.T) {
	validator := NewProfileValidator(fakeValidationStore{
		templateExists: true,
		secrets: map[string]bool{
			"OPENAI_API_KEY": true,
		},
	})

	result, err := validator.Validate(context.Background(), "tenant-1", domain.UpsertTenantProfileRequest{
		TemplateID:     "template-1",
		ResourceTier:   "standard",
		RouteKey:       "tenant-1",
		ModelProvider:  "openai-compatible",
		ModelName:      "gpt-4.1",
		Channels:       json.RawMessage(`[]`),
		Skills:         json.RawMessage(`[]`),
		ExtraFiles:     json.RawMessage(`[]`),
		SoulMarkdown:   "hello",
		MemoryMarkdown: "world",
	})
	if err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if !result.IsValid {
		t.Fatalf("expected valid result, got %+v", result)
	}
}

func TestValidateMissingSecret(t *testing.T) {
	validator := NewProfileValidator(fakeValidationStore{templateExists: true, secrets: map[string]bool{}})

	result, err := validator.Validate(context.Background(), "tenant-1", domain.UpsertTenantProfileRequest{
		TemplateID:     "template-1",
		ResourceTier:   "standard",
		RouteKey:       "tenant-1",
		ModelProvider:  "openai-compatible",
		ModelName:      "gpt-4.1",
		Channels:       json.RawMessage(`[]`),
		Skills:         json.RawMessage(`[]`),
		ExtraFiles:     json.RawMessage(`[]`),
		SoulMarkdown:   "hello",
		MemoryMarkdown: "world",
	})
	if err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if result.IsValid {
		t.Fatalf("expected invalid result")
	}
	if len(result.Errors) == 0 || result.Errors[0].Field != "tenant_secrets.OPENAI_API_KEY" {
		t.Fatalf("expected missing OPENAI_API_KEY error, got %+v", result.Errors)
	}
}
