package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"openclaw-autodeploy/internal/client"
	"openclaw-autodeploy/internal/domain"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

type Options struct {
	Server    string
	Token     string
	TokenFile string
	Output    string
	Timeout   time.Duration
}

func Execute() error {
	return newRootCmd().Execute()
}

func ExitCode(err error) int {
	var apiErr *client.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.Code {
		case "VALIDATION_ERROR", "TENANT_PROFILE_INVALID":
			return 2
		case "UNAUTHORIZED", "FORBIDDEN":
			return 3
		case "TENANT_NOT_FOUND", "TEMPLATE_NOT_FOUND", "DEPLOYMENT_JOB_NOT_FOUND", "INSTANCE_NOT_FOUND":
			return 4
		case "IDEMPOTENCY_CONFLICT", "JOB_ALREADY_RUNNING":
			return 5
		case "DOCKER_BACKEND_UNAVAILABLE", "CAPACITY_EXCEEDED":
			return 6
		default:
			return 10
		}
	}
	return 1
}

func newRootCmd() *cobra.Command {
	options := &Options{
		Server:  strings.TrimSpace(os.Getenv("OPENCLAWCTL_SERVER")),
		Token:   strings.TrimSpace(os.Getenv("OPENCLAWCTL_TOKEN")),
		Output:  defaultOutput(),
		Timeout: 15 * time.Second,
	}

	cmd := &cobra.Command{
		Use:           "openclawctl",
		Short:         "OpenClaw AutoDeploy control plane CLI",
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(_ *cobra.Command, _ []string) error {
			if options.Token == "" && options.TokenFile != "" {
				contents, err := os.ReadFile(options.TokenFile)
				if err != nil {
					return fmt.Errorf("read token file: %w", err)
				}
				options.Token = strings.TrimSpace(string(contents))
			}
			switch options.Output {
			case "json", "yaml":
				return nil
			default:
				return fmt.Errorf("unsupported output format %q", options.Output)
			}
		},
	}

	cmd.PersistentFlags().StringVar(&options.Server, "server", options.Server, "Control plane base URL")
	cmd.PersistentFlags().StringVar(&options.Token, "token", options.Token, "Bearer token")
	cmd.PersistentFlags().StringVar(&options.TokenFile, "token-file", "", "Read bearer token from file")
	cmd.PersistentFlags().StringVar(&options.Output, "output", options.Output, "Output format: json or yaml")
	cmd.PersistentFlags().DurationVar(&options.Timeout, "timeout", options.Timeout, "Request timeout")

	cmd.AddCommand(newHealthCmd(options))
	cmd.AddCommand(newReadyCmd(options))
	cmd.AddCommand(newTenantCmd(options))
	cmd.AddCommand(newProfileCmd(options))
	cmd.AddCommand(newSecretCmd(options))
	cmd.AddCommand(newTemplateCmd(options))
	cmd.AddCommand(newImageCmd(options))
	cmd.AddCommand(newProviderCmd(options))
	cmd.AddCommand(newAPIKeyCmd(options))
	cmd.AddCommand(newDeploymentCmd(options))
	cmd.AddCommand(newJobCmd(options))
	cmd.AddCommand(newInstanceCmd(options))

	return cmd
}

func newHealthCmd(options *Options) *cobra.Command {
	return &cobra.Command{
		Use:   "health",
		Short: "Check API liveness",
		RunE: func(_ *cobra.Command, _ []string) error {
			var response domain.HealthResponse
			if err := mustClient(options).Get(context.Background(), "/healthz", &response); err != nil {
				return err
			}
			return printOutput(response, options.Output)
		},
	}
}

func newReadyCmd(options *Options) *cobra.Command {
	return &cobra.Command{
		Use:   "ready",
		Short: "Check API readiness",
		RunE: func(_ *cobra.Command, _ []string) error {
			var response domain.HealthResponse
			if err := mustClient(options).Get(context.Background(), "/readyz", &response); err != nil {
				return err
			}
			return printOutput(response, options.Output)
		},
	}
}

func newTenantCmd(options *Options) *cobra.Command {
	cmd := &cobra.Command{Use: "tenant", Short: "Manage tenants"}

	var createInput domain.CreateTenantRequest
	create := &cobra.Command{
		Use:   "create",
		Short: "Create a tenant",
		RunE: func(_ *cobra.Command, _ []string) error {
			var response domain.TenantResponse
			if err := mustClient(options).Post(context.Background(), "/api/v1/tenants", createInput, &response); err != nil {
				return err
			}
			return printOutput(response, options.Output)
		},
	}
	create.Flags().StringVar(&createInput.ExternalUserID, "external-user-id", "", "External user ID")
	create.Flags().StringVar(&createInput.Slug, "slug", "", "Tenant slug")
	create.Flags().StringVar(&createInput.DisplayName, "display-name", "", "Display name")
	_ = create.MarkFlagRequired("external-user-id")
	_ = create.MarkFlagRequired("slug")
	_ = create.MarkFlagRequired("display-name")

	var tenantID string
	get := &cobra.Command{
		Use:   "get",
		Short: "Get a tenant",
		RunE: func(_ *cobra.Command, _ []string) error {
			var response domain.TenantResponse
			if err := mustClient(options).Get(context.Background(), "/api/v1/tenants/"+tenantID, &response); err != nil {
				return err
			}
			return printOutput(response, options.Output)
		},
	}
	get.Flags().StringVar(&tenantID, "tenant", "", "Tenant ID")
	_ = get.MarkFlagRequired("tenant")

	var listStatus string
	var listExternalUserID string
	var listSlug string
	list := &cobra.Command{
		Use:   "list",
		Short: "List tenants",
		RunE: func(_ *cobra.Command, _ []string) error {
			query := buildQuery(map[string]string{
				"status":           listStatus,
				"external_user_id": listExternalUserID,
				"slug":             listSlug,
			})
			var response domain.TenantsResponse
			if err := mustClient(options).Get(context.Background(), "/api/v1/tenants"+query, &response); err != nil {
				return err
			}
			return printOutput(response, options.Output)
		},
	}
	list.Flags().StringVar(&listStatus, "status", "", "Filter by status")
	list.Flags().StringVar(&listExternalUserID, "external-user-id", "", "Filter by external user ID")
	list.Flags().StringVar(&listSlug, "slug", "", "Filter by slug")

	cmd.AddCommand(create, get, list)
	return cmd
}

func newProfileCmd(options *Options) *cobra.Command {
	cmd := &cobra.Command{Use: "profile", Short: "Manage tenant profiles"}

	var tenantID string
	get := &cobra.Command{
		Use:   "get",
		Short: "Get a tenant profile",
		RunE: func(_ *cobra.Command, _ []string) error {
			var response domain.ProfileResponse
			if err := mustClient(options).Get(context.Background(), "/api/v1/tenants/"+tenantID+"/profile", &response); err != nil {
				return err
			}
			return printOutput(response, options.Output)
		},
	}
	get.Flags().StringVar(&tenantID, "tenant", "", "Tenant ID")
	_ = get.MarkFlagRequired("tenant")

	var setTenantID string
	var templateID string
	var resourceTier string
	var routeKey string
	var modelProvider string
	var modelName string
	var channelsFile string
	var skillsFile string
	var soulFile string
	var memoryFile string
	var extraFiles []string
	set := &cobra.Command{
		Use:   "set",
		Short: "Create or replace a tenant profile",
		RunE: func(_ *cobra.Command, _ []string) error {
			channels, err := readOptionalJSONFile(channelsFile)
			if err != nil {
				return err
			}
			skills, err := readOptionalJSONFile(skillsFile)
			if err != nil {
				return err
			}
			extraPayload, err := buildExtraFiles(extraFiles)
			if err != nil {
				return err
			}
			soulMarkdown, err := readOptionalTextFile(soulFile)
			if err != nil {
				return err
			}
			memoryMarkdown, err := readOptionalTextFile(memoryFile)
			if err != nil {
				return err
			}

			input := domain.UpsertTenantProfileRequest{
				TemplateID:     templateID,
				ResourceTier:   resourceTier,
				RouteKey:       routeKey,
				ModelProvider:  modelProvider,
				ModelName:      modelName,
				Channels:       channels,
				Skills:         skills,
				SoulMarkdown:   soulMarkdown,
				MemoryMarkdown: memoryMarkdown,
				ExtraFiles:     extraPayload,
			}

			var response domain.ProfileResponse
			if err := mustClient(options).Put(context.Background(), "/api/v1/tenants/"+setTenantID+"/profile", input, &response); err != nil {
				return err
			}
			return printOutput(response, options.Output)
		},
	}
	set.Flags().StringVar(&setTenantID, "tenant", "", "Tenant ID")
	set.Flags().StringVar(&templateID, "template", "", "Template ID")
	set.Flags().StringVar(&resourceTier, "tier", "", "Resource tier")
	set.Flags().StringVar(&routeKey, "route-key", "", "Route key")
	set.Flags().StringVar(&modelProvider, "model-provider", "", "Model provider")
	set.Flags().StringVar(&modelName, "model-name", "", "Model name")
	set.Flags().StringVar(&channelsFile, "channels-file", "", "Path to channels JSON")
	set.Flags().StringVar(&skillsFile, "skills-file", "", "Path to skills JSON")
	set.Flags().StringVar(&soulFile, "soul-file", "", "Path to SOUL.md")
	set.Flags().StringVar(&memoryFile, "memory-file", "", "Path to memory.md")
	set.Flags().StringArrayVar(&extraFiles, "extra-file", nil, "Extra file mapping path=local-file")
	_ = set.MarkFlagRequired("tenant")
	_ = set.MarkFlagRequired("template")
	_ = set.MarkFlagRequired("tier")
	_ = set.MarkFlagRequired("route-key")
	_ = set.MarkFlagRequired("model-provider")
	_ = set.MarkFlagRequired("model-name")

	var validateTenantID string
	validate := &cobra.Command{
		Use:   "validate",
		Short: "Validate a tenant profile",
		RunE: func(_ *cobra.Command, _ []string) error {
			var response domain.ValidationResponse
			if err := mustClient(options).Post(context.Background(), "/api/v1/tenants/"+validateTenantID+"/profile/validate", map[string]any{}, &response); err != nil {
				return err
			}
			return printOutput(response, options.Output)
		},
	}
	validate.Flags().StringVar(&validateTenantID, "tenant", "", "Tenant ID")
	_ = validate.MarkFlagRequired("tenant")

	cmd.AddCommand(get, set, validate)
	return cmd
}

func newSecretCmd(options *Options) *cobra.Command {
	cmd := &cobra.Command{Use: "secret", Short: "Manage tenant secrets"}

	var listTenantID string
	list := &cobra.Command{
		Use:   "list",
		Short: "List tenant secret metadata",
		RunE: func(_ *cobra.Command, _ []string) error {
			var response domain.SecretsResponse
			if err := mustClient(options).Get(context.Background(), "/api/v1/tenants/"+listTenantID+"/secrets", &response); err != nil {
				return err
			}
			return printOutput(response, options.Output)
		},
	}
	list.Flags().StringVar(&listTenantID, "tenant", "", "Tenant ID")
	_ = list.MarkFlagRequired("tenant")

	var setTenantID string
	var fromEnv string
	var fromFile string
	var useStdin bool
	var secretType string
	set := &cobra.Command{
		Use:   "set SECRET_KEY",
		Short: "Create or rotate a secret",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			value, err := resolveSecretValue(fromEnv, fromFile, useStdin)
			if err != nil {
				return err
			}
			var response domain.SecretResponse
			if err := mustClient(options).Put(context.Background(), "/api/v1/tenants/"+setTenantID+"/secrets/"+args[0], domain.SetSecretRequest{
				Value:      value,
				SecretType: secretType,
			}, &response); err != nil {
				return err
			}
			return printOutput(response, options.Output)
		},
	}
	set.Flags().StringVar(&setTenantID, "tenant", "", "Tenant ID")
	set.Flags().StringVar(&fromEnv, "from-env", "", "Read secret from environment variable")
	set.Flags().StringVar(&fromFile, "from-file", "", "Read secret from file")
	set.Flags().BoolVar(&useStdin, "stdin", false, "Read secret from stdin")
	set.Flags().StringVar(&secretType, "secret-type", "api_key", "Secret type")
	_ = set.MarkFlagRequired("tenant")

	var deleteTenantID string
	var yes bool
	deleteCmd := &cobra.Command{
		Use:   "delete SECRET_KEY",
		Short: "Revoke a secret",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if !yes {
				return fmt.Errorf("pass --yes to confirm secret deletion")
			}
			if err := mustClient(options).Delete(context.Background(), "/api/v1/tenants/"+deleteTenantID+"/secrets/"+args[0], nil, nil); err != nil {
				return err
			}
			_, _ = fmt.Fprintln(os.Stdout, "secret deleted")
			return nil
		},
	}
	deleteCmd.Flags().StringVar(&deleteTenantID, "tenant", "", "Tenant ID")
	deleteCmd.Flags().BoolVar(&yes, "yes", false, "Confirm deletion")
	_ = deleteCmd.MarkFlagRequired("tenant")

	cmd.AddCommand(list, set, deleteCmd)
	return cmd
}

func newTemplateCmd(options *Options) *cobra.Command {
	cmd := &cobra.Command{Use: "template", Short: "Query deployment templates"}

	list := &cobra.Command{
		Use:   "list",
		Short: "List templates",
		RunE: func(_ *cobra.Command, _ []string) error {
			var response domain.TemplatesResponse
			if err := mustClient(options).Get(context.Background(), "/api/v1/templates", &response); err != nil {
				return err
			}
			return printOutput(response, options.Output)
		},
	}

	var templateID string
	get := &cobra.Command{
		Use:   "get",
		Short: "Get one template",
		RunE: func(_ *cobra.Command, _ []string) error {
			var response domain.TemplateResponse
			if err := mustClient(options).Get(context.Background(), "/api/v1/templates/"+templateID, &response); err != nil {
				return err
			}
			return printOutput(response, options.Output)
		},
	}
	get.Flags().StringVar(&templateID, "template", "", "Template ID")
	_ = get.MarkFlagRequired("template")

	cmd.AddCommand(list, get)
	return cmd
}

func newImageCmd(options *Options) *cobra.Command {
	var status string
	var imageRef string
	cmd := &cobra.Command{
		Use:   "image",
		Short: "Query available images",
		RunE: func(_ *cobra.Command, _ []string) error {
			query := buildQuery(map[string]string{"status": status, "image_ref": imageRef})
			var response domain.ImagesResponse
			if err := mustClient(options).Get(context.Background(), "/api/v1/images"+query, &response); err != nil {
				return err
			}
			return printOutput(response, options.Output)
		},
	}
	cmd.Flags().StringVar(&status, "status", "", "Filter by image status")
	cmd.Flags().StringVar(&imageRef, "image-ref", "", "Filter by image reference")
	return cmd
}

func newProviderCmd(options *Options) *cobra.Command {
	cmd := &cobra.Command{Use: "provider", Short: "Manage LLM providers"}

	list := &cobra.Command{
		Use:   "list",
		Short: "List all LLM providers",
		RunE: func(_ *cobra.Command, _ []string) error {
			var response domain.LLMProvidersResponse
			if err := mustClient(options).Get(context.Background(), "/api/v1/providers", &response); err != nil {
				return err
			}
			return printOutput(response, options.Output)
		},
	}

	var providerID string
	get := &cobra.Command{
		Use:   "get",
		Short: "Get one provider",
		RunE: func(_ *cobra.Command, _ []string) error {
			var response domain.LLMProviderResponse
			if err := mustClient(options).Get(context.Background(), "/api/v1/providers/"+providerID, &response); err != nil {
				return err
			}
			return printOutput(response, options.Output)
		},
	}
	get.Flags().StringVar(&providerID, "provider", "", "Provider ID")
	_ = get.MarkFlagRequired("provider")

	var upsertName string
	var upsertDisplayName string
	var upsertDescription string
	var upsertBaseURL string
	var upsertStatus string
	upsert := &cobra.Command{
		Use:   "upsert",
		Short: "Create or update a provider",
		RunE: func(_ *cobra.Command, _ []string) error {
			var response domain.LLMProviderResponse
			if err := mustClient(options).Post(context.Background(), "/api/v1/providers", domain.UpsertLLMProviderRequest{
				Name:        upsertName,
				DisplayName: upsertDisplayName,
				Description: upsertDescription,
				BaseURL:     upsertBaseURL,
				Status:      upsertStatus,
			}, &response); err != nil {
				return err
			}
			return printOutput(response, options.Output)
		},
	}
	upsert.Flags().StringVar(&upsertName, "name", "", "Provider name (unique, e.g. minimax)")
	upsert.Flags().StringVar(&upsertDisplayName, "display-name", "", "Display name")
	upsert.Flags().StringVar(&upsertDescription, "description", "", "Description")
	upsert.Flags().StringVar(&upsertBaseURL, "base-url", "", "API base URL")
	upsert.Flags().StringVar(&upsertStatus, "status", "active", "Status (active/inactive)")
	_ = upsert.MarkFlagRequired("name")

	var deleteProviderID string
	var yes bool
	delete := &cobra.Command{
		Use:   "delete",
		Short: "Delete a provider",
		RunE: func(_ *cobra.Command, _ []string) error {
			if !yes {
				return fmt.Errorf("pass --yes to confirm deletion")
			}
			if err := mustClient(options).Delete(context.Background(), "/api/v1/providers/"+deleteProviderID, nil, nil); err != nil {
				return err
			}
			_, _ = fmt.Fprintln(os.Stdout, "provider deleted")
			return nil
		},
	}
	delete.Flags().StringVar(&deleteProviderID, "provider", "", "Provider ID")
	delete.Flags().BoolVar(&yes, "yes", false, "Confirm deletion")
	_ = delete.MarkFlagRequired("provider")

	cmd.AddCommand(list, get, upsert, delete)
	return cmd
}

func newAPIKeyCmd(options *Options) *cobra.Command {
	cmd := &cobra.Command{Use: "apikey", Short: "Manage LLM API keys"}

	var listProviderID string
	list := &cobra.Command{
		Use:   "list",
		Short: "List API keys",
		RunE: func(_ *cobra.Command, _ []string) error {
			query := buildQuery(map[string]string{"provider_id": listProviderID})
			var response domain.LLMAPIKeysResponse
			if err := mustClient(options).Get(context.Background(), "/api/v1/api-keys"+query, &response); err != nil {
				return err
			}
			return printOutput(response, options.Output)
		},
	}
	list.Flags().StringVar(&listProviderID, "provider-id", "", "Filter by provider ID")

	var addProviderID string
	var addValue string
	var addStatus string
	add := &cobra.Command{
		Use:   "add",
		Short: "Add an API key to a provider",
		RunE: func(_ *cobra.Command, _ []string) error {
			var response domain.LLMAPIKeyResponse
			if err := mustClient(options).Post(context.Background(), "/api/v1/api-keys", domain.AddLLMAPIKeyRequest{
				ProviderID: addProviderID,
				Value:      addValue,
				Status:     addStatus,
			}, &response); err != nil {
				return err
			}
			return printOutput(response, options.Output)
		},
	}
	add.Flags().StringVar(&addProviderID, "provider-id", "", "Provider ID")
	add.Flags().StringVar(&addValue, "value", "", "API key value")
	add.Flags().StringVar(&addStatus, "status", "active", "Status (active/inactive)")
	_ = add.MarkFlagRequired("provider-id")
	_ = add.MarkFlagRequired("value")

	var getKeyID string
	get := &cobra.Command{
		Use:   "get",
		Short: "Get one API key",
		RunE: func(_ *cobra.Command, _ []string) error {
			var response domain.LLMAPIKeyResponse
			if err := mustClient(options).Get(context.Background(), "/api/v1/api-keys/"+getKeyID, &response); err != nil {
				return err
			}
			return printOutput(response, options.Output)
		},
	}
	get.Flags().StringVar(&getKeyID, "key", "", "API key ID")
	_ = get.MarkFlagRequired("key")

	var deactivateKeyID string
	var yesDeactivate bool
	deactivate := &cobra.Command{
		Use:   "deactivate",
		Short: "Deactivate an API key",
		RunE: func(_ *cobra.Command, _ []string) error {
			if !yesDeactivate {
				return fmt.Errorf("pass --yes to confirm deactivation")
			}
			if err := mustClient(options).Delete(context.Background(), "/api/v1/api-keys/"+deactivateKeyID, nil, nil); err != nil {
				return err
			}
			_, _ = fmt.Fprintln(os.Stdout, "api key deactivated")
			return nil
		},
	}
	deactivate.Flags().StringVar(&deactivateKeyID, "key", "", "API key ID")
	deactivate.Flags().BoolVar(&yesDeactivate, "yes", false, "Confirm deactivation")
	_ = deactivate.MarkFlagRequired("key")

	cmd.AddCommand(list, add, get, deactivate)
	return cmd
}

func newDeploymentCmd(options *Options) *cobra.Command {
	cmd := &cobra.Command{Use: "deployment", Short: "Manage tenant deployment lifecycle"}
	cmd.AddCommand(newDeploymentActionCmd(options, "deploy"))
	cmd.AddCommand(newDeploymentActionCmd(options, "redeploy"))
	cmd.AddCommand(newDeploymentActionCmd(options, "stop"))
	cmd.AddCommand(newDeploymentActionCmd(options, "start"))
	cmd.AddCommand(newDeploymentActionCmd(options, "restart"))

	var tenantID string
	var destroyWorkspace bool
	var destroyVolume bool
	var yes bool
	destroy := &cobra.Command{
		Use:   "destroy",
		Short: "Destroy the active deployment",
		RunE: func(_ *cobra.Command, _ []string) error {
			if !yes {
				return fmt.Errorf("pass --yes to confirm deployment destruction")
			}
			payload := domain.DeploymentActionRequest{DestroyWorkspace: destroyWorkspace, DestroyVolume: destroyVolume}
			var response domain.DeploymentJobResponse
			if err := mustClient(options).Delete(context.Background(), "/api/v1/tenants/"+tenantID+"/deployment", payload, &response); err != nil {
				return err
			}
			return printOutput(response, options.Output)
		},
	}
	destroy.Flags().StringVar(&tenantID, "tenant", "", "Tenant ID")
	destroy.Flags().BoolVar(&destroyWorkspace, "destroy-workspace", false, "Delete workspace files")
	destroy.Flags().BoolVar(&destroyVolume, "destroy-volume", false, "Delete Docker volume")
	destroy.Flags().BoolVar(&yes, "yes", false, "Confirm destruction")
	_ = destroy.MarkFlagRequired("tenant")
	cmd.AddCommand(destroy)
	return cmd
}

func newDeploymentActionCmd(options *Options, action string) *cobra.Command {
	var tenantID string
	var reason string
	var strategy string
	command := &cobra.Command{
		Use:   action,
		Short: strings.Title(action) + " a tenant deployment",
		RunE: func(_ *cobra.Command, _ []string) error {
			payload := domain.DeploymentActionRequest{Reason: reason, Strategy: strategy}
			var response domain.DeploymentJobResponse
			if err := mustClient(options).Post(context.Background(), "/api/v1/tenants/"+tenantID+"/"+action, payload, &response); err != nil {
				return err
			}
			return printOutput(response, options.Output)
		},
	}
	command.Flags().StringVar(&tenantID, "tenant", "", "Tenant ID")
	command.Flags().StringVar(&reason, "reason", "", "Operation reason")
	command.Flags().StringVar(&strategy, "strategy", "", "Deployment strategy")
	_ = command.MarkFlagRequired("tenant")
	return command
}

func newJobCmd(options *Options) *cobra.Command {
	cmd := &cobra.Command{Use: "job", Short: "Query deployment jobs"}

	var jobID string
	get := &cobra.Command{
		Use:   "get",
		Short: "Get a deployment job",
		RunE: func(_ *cobra.Command, _ []string) error {
			var response domain.DeploymentJobResponse
			if err := mustClient(options).Get(context.Background(), "/api/v1/jobs/"+jobID, &response); err != nil {
				return err
			}
			return printOutput(response, options.Output)
		},
	}
	get.Flags().StringVar(&jobID, "job", "", "Job ID")
	_ = get.MarkFlagRequired("job")

	var tenantID string
	var status string
	var jobType string
	list := &cobra.Command{
		Use:   "list",
		Short: "List deployment jobs",
		RunE: func(_ *cobra.Command, _ []string) error {
			query := buildQuery(map[string]string{"tenant_id": tenantID, "status": status, "job_type": jobType})
			var response domain.DeploymentJobsResponse
			if err := mustClient(options).Get(context.Background(), "/api/v1/jobs"+query, &response); err != nil {
				return err
			}
			return printOutput(response, options.Output)
		},
	}
	list.Flags().StringVar(&tenantID, "tenant", "", "Tenant ID filter")
	list.Flags().StringVar(&status, "status", "", "Status filter")
	list.Flags().StringVar(&jobType, "job-type", "", "Job type filter")

	var watchJobID string
	var interval time.Duration
	var watchTimeout time.Duration
	watch := &cobra.Command{
		Use:   "watch",
		Short: "Watch a job until completion",
		RunE: func(_ *cobra.Command, _ []string) error {
			deadline := time.NewTimer(watchTimeout)
			defer deadline.Stop()
			ticker := time.NewTicker(interval)
			defer ticker.Stop()
			for {
				var response domain.DeploymentJobResponse
				if err := mustClient(options).Get(context.Background(), "/api/v1/jobs/"+watchJobID, &response); err != nil {
					return err
				}
				if response.Job.Status == "succeeded" || response.Job.Status == "failed" || response.Job.Status == "cancelled" {
					return printOutput(response, options.Output)
				}
				select {
				case <-deadline.C:
					return fmt.Errorf("timed out waiting for job completion")
				case <-ticker.C:
				}
			}
		},
	}
	watch.Flags().StringVar(&watchJobID, "job", "", "Job ID")
	watch.Flags().DurationVar(&interval, "interval", 2*time.Second, "Polling interval")
	watch.Flags().DurationVar(&watchTimeout, "watch-timeout", 2*time.Minute, "Watch timeout")
	_ = watch.MarkFlagRequired("job")

	cmd.AddCommand(get, list, watch)
	return cmd
}

func newInstanceCmd(options *Options) *cobra.Command {
	cmd := &cobra.Command{Use: "instance", Short: "Query tenant instances"}

	var tenantID string
	get := &cobra.Command{
		Use:   "get",
		Short: "Get the current tenant instance",
		RunE: func(_ *cobra.Command, _ []string) error {
			var response domain.TenantInstanceResponse
			if err := mustClient(options).Get(context.Background(), "/api/v1/tenants/"+tenantID+"/instance", &response); err != nil {
				return err
			}
			return printOutput(response, options.Output)
		},
	}
	get.Flags().StringVar(&tenantID, "tenant", "", "Tenant ID")
	_ = get.MarkFlagRequired("tenant")

	var listTenantID string
	history := &cobra.Command{
		Use:   "history",
		Short: "List tenant instance history",
		RunE: func(_ *cobra.Command, _ []string) error {
			var response domain.TenantInstancesResponse
			if err := mustClient(options).Get(context.Background(), "/api/v1/tenants/"+listTenantID+"/instances", &response); err != nil {
				return err
			}
			return printOutput(response, options.Output)
		},
	}
	history.Flags().StringVar(&listTenantID, "tenant", "", "Tenant ID")
	_ = history.MarkFlagRequired("tenant")

	cmd.AddCommand(get, history)
	return cmd
}

func mustClient(options *Options) *client.Client {
	return client.New(strings.TrimSpace(options.Server), strings.TrimSpace(options.Token), options.Timeout)
}

func buildQuery(values map[string]string) string {
	parts := make([]string, 0, len(values))
	for key, value := range values {
		if strings.TrimSpace(value) == "" {
			continue
		}
		parts = append(parts, url.QueryEscape(key)+"="+url.QueryEscape(value))
	}
	if len(parts) == 0 {
		return ""
	}
	return "?" + strings.Join(parts, "&")
}

func printOutput(payload any, format string) error {
	jsonBytes, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal output: %w", err)
	}
	if format == "json" {
		_, err = fmt.Fprintln(os.Stdout, string(jsonBytes))
		return err
	}

	var generic any
	if err := json.Unmarshal(jsonBytes, &generic); err != nil {
		return fmt.Errorf("normalize output: %w", err)
	}
	yamlBytes, err := yaml.Marshal(generic)
	if err != nil {
		return fmt.Errorf("marshal yaml output: %w", err)
	}
	_, err = fmt.Fprintln(os.Stdout, strings.TrimSpace(string(yamlBytes)))
	return err
}

func defaultOutput() string {
	value := strings.TrimSpace(os.Getenv("OPENCLAWCTL_OUTPUT"))
	if value == "" {
		return "yaml"
	}
	return value
}

func resolveSecretValue(fromEnv string, fromFile string, useStdin bool) (string, error) {
	sources := 0
	if fromEnv != "" {
		sources++
	}
	if fromFile != "" {
		sources++
	}
	if useStdin {
		sources++
	}
	if sources != 1 {
		return "", fmt.Errorf("use exactly one of --from-env, --from-file, or --stdin")
	}

	if fromEnv != "" {
		value, ok := os.LookupEnv(fromEnv)
		if !ok || strings.TrimSpace(value) == "" {
			return "", fmt.Errorf("environment variable %q is empty", fromEnv)
		}
		return strings.TrimSpace(value), nil
	}
	if fromFile != "" {
		contents, err := os.ReadFile(fromFile)
		if err != nil {
			return "", fmt.Errorf("read secret file: %w", err)
		}
		value := strings.TrimSpace(string(contents))
		if value == "" {
			return "", fmt.Errorf("secret file is empty")
		}
		return value, nil
	}
	reader := bufio.NewReader(os.Stdin)
	contents, err := io.ReadAll(reader)
	if err != nil {
		return "", fmt.Errorf("read secret from stdin: %w", err)
	}
	value := strings.TrimSpace(string(contents))
	if value == "" {
		return "", fmt.Errorf("stdin secret is empty")
	}
	return value, nil
}

func readOptionalJSONFile(path string) (json.RawMessage, error) {
	if strings.TrimSpace(path) == "" {
		return json.RawMessage(`[]`), nil
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read json file %q: %w", path, err)
	}
	trimmed := strings.TrimSpace(string(contents))
	if trimmed == "" {
		return json.RawMessage(`[]`), nil
	}
	if !json.Valid([]byte(trimmed)) {
		return nil, fmt.Errorf("invalid json in %q", path)
	}
	return json.RawMessage(trimmed), nil
}

func readOptionalTextFile(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", nil
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read file %q: %w", path, err)
	}
	return string(contents), nil
}

func buildExtraFiles(items []string) (json.RawMessage, error) {
	type extraFile struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	result := make([]extraFile, 0, len(items))
	for _, item := range items {
		parts := strings.SplitN(item, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid --extra-file %q, expected path=local-file", item)
		}
		contents, err := os.ReadFile(filepath.Clean(parts[1]))
		if err != nil {
			return nil, fmt.Errorf("read extra file %q: %w", parts[1], err)
		}
		result = append(result, extraFile{Path: parts[0], Content: string(contents)})
	}
	if len(result) == 0 {
		return json.RawMessage(`[]`), nil
	}
	payload, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("marshal extra files: %w", err)
	}
	return payload, nil
}
