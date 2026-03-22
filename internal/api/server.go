package api

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"openclaw-autodeploy/internal/db"
	"openclaw-autodeploy/internal/domain"
	storepkg "openclaw-autodeploy/internal/store/postgres"
	"openclaw-autodeploy/internal/service"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5"
)

type Server struct {
	store        *storepkg.Store
	validator    *service.ProfileValidator
	docker       dockerPinger
	workerName   string
	heartbeatTTL time.Duration
	apiToken     string
}

type dockerPinger interface {
	Ping(ctx context.Context) error
}

func New(store *storepkg.Store, validator *service.ProfileValidator, docker dockerPinger, workerName string, heartbeatTTL time.Duration, apiToken string) *Server {
	return &Server{
		store:        store,
		validator:    validator,
		docker:       docker,
		workerName:   workerName,
		heartbeatTTL: heartbeatTTL,
		apiToken:     apiToken,
	}
}

func authMiddleware(token string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if token == "" {
				next.ServeHTTP(w, r)
				return
			}
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				writeError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "missing Authorization header", nil)
				return
			}
			if !strings.HasPrefix(authHeader, "Bearer ") {
				writeError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "invalid Authorization header format", nil)
				return
			}
			provided := strings.TrimPrefix(authHeader, "Bearer ")
			if provided != token {
				writeError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "invalid token", nil)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func (s *Server) Routes() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)

	authMw := authMiddleware(s.apiToken)

	r.Get("/healthz", s.handleHealth)
	r.Get("/metrics", s.handleMetrics)

	if s.apiToken != "" {
		r.Group(func(r chi.Router) {
			r.Use(authMw)
			r.Get("/readyz", s.handleReady)
		})
	} else {
		r.Get("/readyz", s.handleReady)
	}

	r.Route("/api/v1", func(r chi.Router) {
		r.Use(authMw)
		r.Post("/tenants", s.handleCreateTenant)
		r.Get("/tenants", s.handleListTenants)
		r.Get("/tenants/{tenantID}", s.handleGetTenant)

		r.Get("/tenants/{tenantID}/profile", s.handleGetProfile)
		r.Put("/tenants/{tenantID}/profile", s.handlePutProfile)
		r.Post("/tenants/{tenantID}/profile/validate", s.handleValidateProfile)

		r.Get("/tenants/{tenantID}/secrets", s.handleListSecrets)
		r.Put("/tenants/{tenantID}/secrets/{secretKey}", s.handleSetSecret)
		r.Delete("/tenants/{tenantID}/secrets/{secretKey}", s.handleDeleteSecret)

		r.Get("/templates", s.handleListTemplates)
		r.Get("/templates/{templateID}", s.handleGetTemplate)
		r.Get("/images", s.handleListImages)

		r.Post("/tenants/{tenantID}/deploy", s.handleDeploy)
		r.Post("/tenants/{tenantID}/redeploy", s.handleRedeploy)
		r.Post("/tenants/{tenantID}/stop", s.handleStop)
		r.Post("/tenants/{tenantID}/start", s.handleStart)
		r.Post("/tenants/{tenantID}/restart", s.handleRestart)
		r.Delete("/tenants/{tenantID}/deployment", s.handleDestroy)

		r.Get("/jobs/{jobID}", s.handleGetJob)
		r.Get("/jobs", s.handleListJobs)
		r.Get("/tenants/{tenantID}/instance", s.handleGetInstance)
		r.Get("/tenants/{tenantID}/instances", s.handleListInstances)

		r.Get("/providers", s.handleListProviders)
		r.Post("/providers", s.handleUpsertProvider)
		r.Get("/providers/{providerID}", s.handleGetProvider)
		r.Delete("/providers/{providerID}", s.handleDeleteProvider)

		r.Get("/api-keys", s.handleListAPIKeys)
		r.Post("/api-keys", s.handleAddAPIKey)
		r.Get("/api-keys/{keyID}", s.handleGetAPIKey)
		r.Delete("/api-keys/{keyID}", s.handleDeactivateAPIKey)

		r.Get("/tenants/{tenantID}/llm-allocation", s.handleGetTenantLLMAllocation)
		r.Post("/tenants/{tenantID}/llm-allocation", s.handleAllocateLLMKey)
		r.Delete("/tenants/{tenantID}/llm-allocation", s.handleRevokeLLMAllocation)
	})

	return r
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, domain.HealthResponse{Status: "ok"})
}

func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	tenants, _ := s.store.CountTenants(ctx)
	containersRunning, _ := s.store.CountRunningContainers(ctx)
	jobsPending, _ := s.store.CountPendingJobs(ctx)
	jobsSucceeded, _ := s.store.CountJobsByStatus(ctx, "succeeded")
	jobsFailed, _ := s.store.CountJobsByStatus(ctx, "failed")

	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	fmt.Fprintf(w, "# HELP openclaw_up Control plane is up.\n")
	fmt.Fprintf(w, "# TYPE openclaw_up gauge\n")
	fmt.Fprintf(w, "openclaw_up 1\n")
	fmt.Fprintf(w, "# HELP openclaw_tenants_total Total number of tenants.\n")
	fmt.Fprintf(w, "# TYPE openclaw_tenants_total gauge\n")
	fmt.Fprintf(w, "openclaw_tenants_total %d\n", tenants)
	fmt.Fprintf(w, "# HELP openclaw_containers_running Number of running tenant containers.\n")
	fmt.Fprintf(w, "# TYPE openclaw_containers_running gauge\n")
	fmt.Fprintf(w, "openclaw_containers_running %d\n", containersRunning)
	fmt.Fprintf(w, "# HELP openclaw_jobs_pending Number of pending jobs.\n")
	fmt.Fprintf(w, "# TYPE openclaw_jobs_pending gauge\n")
	fmt.Fprintf(w, "openclaw_jobs_pending %d\n", jobsPending)
	fmt.Fprintf(w, "# HELP openclaw_jobs_succeeded_total Total succeeded jobs.\n")
	fmt.Fprintf(w, "# TYPE openclaw_jobs_succeeded_total counter\n")
	fmt.Fprintf(w, "openclaw_jobs_succeeded_total %d\n", jobsSucceeded)
	fmt.Fprintf(w, "# HELP openclaw_jobs_failed_total Total failed jobs.\n")
	fmt.Fprintf(w, "# TYPE openclaw_jobs_failed_total counter\n")
	fmt.Fprintf(w, "openclaw_jobs_failed_total %d\n", jobsFailed)
}

func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
	checks := map[string]string{}
	statusCode := http.StatusOK

	if err := s.store.Ping(r.Context()); err != nil {
		checks["database"] = "error"
		statusCode = http.StatusServiceUnavailable
	} else {
		checks["database"] = "ok"
	}
	if s.docker != nil {
		if err := s.docker.Ping(r.Context()); err != nil {
			checks["docker"] = "error"
			statusCode = http.StatusServiceUnavailable
		} else {
			checks["docker"] = "ok"
		}
	}

	lastSeen, err := s.store.GetWorkerHeartbeat(r.Context(), s.workerName)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			checks["worker"] = "stale"
		} else {
			checks["worker"] = "error"
		}
		statusCode = http.StatusServiceUnavailable
	} else if time.Since(lastSeen) > s.heartbeatTTL {
		checks["worker"] = "stale"
		statusCode = http.StatusServiceUnavailable
	} else {
		checks["worker"] = "ok"
	}

	status := "ready"
	if statusCode != http.StatusOK {
		status = "not_ready"
	}
	writeJSON(w, statusCode, domain.HealthResponse{Status: status, Checks: checks})
}

func (s *Server) handleCreateTenant(w http.ResponseWriter, r *http.Request) {
	var input domain.CreateTenantRequest
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", err.Error(), nil)
		return
	}
	if strings.TrimSpace(input.ExternalUserID) == "" || strings.TrimSpace(input.Slug) == "" || strings.TrimSpace(input.DisplayName) == "" {
		writeError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "external_user_id, slug, and display_name are required", nil)
		return
	}
	tenant, err := s.store.CreateTenant(r.Context(), input)
	if err != nil {
		if db.IsUniqueViolation(err) {
			writeError(w, r, http.StatusConflict, "VALIDATION_ERROR", "tenant already exists", nil)
			return
		}
		writeInternalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, domain.TenantResponse{Tenant: tenant})
}

func (s *Server) handleGetTenant(w http.ResponseWriter, r *http.Request) {
	tenant, err := s.store.GetTenant(r.Context(), chi.URLParam(r, "tenantID"))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, r, http.StatusNotFound, "TENANT_NOT_FOUND", "tenant not found", nil)
			return
		}
		writeInternalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, domain.TenantResponse{Tenant: tenant})
}

func (s *Server) handleListTenants(w http.ResponseWriter, r *http.Request) {
	page, pageSize := parsePagination(r)
	tenants, err := s.store.ListTenants(r.Context(), domain.TenantFilter{
		Status:         r.URL.Query().Get("status"),
		ExternalUserID: r.URL.Query().Get("external_user_id"),
		Slug:           r.URL.Query().Get("slug"),
		Page:           page,
		PageSize:       pageSize,
	})
	if err != nil {
		writeInternalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, domain.TenantsResponse{Tenants: tenants})
}

func (s *Server) handlePutProfile(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	if _, err := s.store.GetTenant(r.Context(), tenantID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, r, http.StatusNotFound, "TENANT_NOT_FOUND", "tenant not found", nil)
			return
		}
		writeInternalError(w, r, err)
		return
	}

	var input domain.UpsertTenantProfileRequest
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", err.Error(), nil)
		return
	}
	validation, err := s.validator.Validate(r.Context(), tenantID, input)
	if err != nil {
		writeInternalError(w, r, err)
		return
	}
	profile, err := s.store.UpsertTenantProfile(r.Context(), tenantID, input, validation)
	if err != nil {
		if db.IsUniqueViolation(err) {
			writeError(w, r, http.StatusConflict, "VALIDATION_ERROR", "route_key already exists", nil)
			return
		}
		if db.IsForeignKeyViolation(err) {
			writeError(w, r, http.StatusNotFound, "TENANT_NOT_FOUND", "tenant or template not found", nil)
			return
		}
		writeInternalError(w, r, err)
		return
	}

	if input.ModelProvider != "" && input.ModelName != "" {
		keys, err := s.store.ListActiveLLMAPIKeysByProvider(r.Context(), input.ModelProvider)
		if err == nil && len(keys) > 0 {
			s.store.AllocateLLMKeyToTenant(r.Context(), tenantID, keys[0].ID, input.ModelName)
		}
	}

	writeJSON(w, http.StatusOK, domain.ProfileResponse{Profile: profile})
}

func (s *Server) handleGetProfile(w http.ResponseWriter, r *http.Request) {
	profile, err := s.store.GetTenantProfile(r.Context(), chi.URLParam(r, "tenantID"))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, r, http.StatusNotFound, "TENANT_PROFILE_INVALID", "tenant profile not found", nil)
			return
		}
		writeInternalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, domain.ProfileResponse{Profile: profile})
}

func (s *Server) handleValidateProfile(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	profile, err := s.store.GetTenantProfile(r.Context(), tenantID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, r, http.StatusNotFound, "TENANT_PROFILE_INVALID", "tenant profile not found", nil)
			return
		}
		writeInternalError(w, r, err)
		return
	}
	validation, err := s.validator.Validate(r.Context(), tenantID, profile.ToUpsertRequest())
	if err != nil {
		writeInternalError(w, r, err)
		return
	}
	profile, err = s.store.UpsertTenantProfile(r.Context(), tenantID, profile.ToUpsertRequest(), validation)
	if err != nil {
		writeInternalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, domain.ValidationResponse{Validation: profile.Validation})
}

func (s *Server) handleSetSecret(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	if _, err := s.store.GetTenant(r.Context(), tenantID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, r, http.StatusNotFound, "TENANT_NOT_FOUND", "tenant not found", nil)
			return
		}
		writeInternalError(w, r, err)
		return
	}

	var input domain.SetSecretRequest
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", err.Error(), nil)
		return
	}
	if strings.TrimSpace(input.Value) == "" {
		writeError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "value is required", nil)
		return
	}

	sum := sha256.Sum256([]byte(input.Value))
	fingerprint := hex.EncodeToString(sum[:])[:8]
	secret, err := s.store.SetTenantSecret(r.Context(), tenantID, chi.URLParam(r, "secretKey"), input, fingerprint)
	if err != nil {
		writeInternalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, domain.SecretResponse{Secret: secret})
}

func (s *Server) handleListSecrets(w http.ResponseWriter, r *http.Request) {
	secrets, err := s.store.ListTenantSecrets(r.Context(), chi.URLParam(r, "tenantID"))
	if err != nil {
		writeInternalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, domain.SecretsResponse{Secrets: secrets})
}

func (s *Server) handleDeleteSecret(w http.ResponseWriter, r *http.Request) {
	deleted, err := s.store.DeleteTenantSecret(r.Context(), chi.URLParam(r, "tenantID"), chi.URLParam(r, "secretKey"))
	if err != nil {
		writeInternalError(w, r, err)
		return
	}
	if !deleted {
		writeError(w, r, http.StatusNotFound, "VALIDATION_ERROR", "secret not found", nil)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleListTemplates(w http.ResponseWriter, r *http.Request) {
	templates, err := s.store.ListTemplates(r.Context())
	if err != nil {
		writeInternalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, domain.TemplatesResponse{Templates: templates})
}

func (s *Server) handleGetTemplate(w http.ResponseWriter, r *http.Request) {
	template, err := s.store.GetTemplate(r.Context(), chi.URLParam(r, "templateID"))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, r, http.StatusNotFound, "TEMPLATE_NOT_FOUND", "template not found", nil)
			return
		}
		writeInternalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, domain.TemplateResponse{Template: template})
}

func (s *Server) handleListImages(w http.ResponseWriter, r *http.Request) {
	images, err := s.store.ListImages(r.Context(), r.URL.Query().Get("status"), r.URL.Query().Get("image_ref"))
	if err != nil {
		writeInternalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, domain.ImagesResponse{Images: images})
}

func (s *Server) handleDeploy(w http.ResponseWriter, r *http.Request) {
	s.handleLifecycleEnqueue(w, r, "deploy")
}

func (s *Server) handleRedeploy(w http.ResponseWriter, r *http.Request) {
	s.handleLifecycleEnqueue(w, r, "redeploy")
}

func (s *Server) handleStop(w http.ResponseWriter, r *http.Request) {
	s.handleLifecycleEnqueue(w, r, "stop")
}

func (s *Server) handleStart(w http.ResponseWriter, r *http.Request) {
	s.handleLifecycleEnqueue(w, r, "start")
}

func (s *Server) handleRestart(w http.ResponseWriter, r *http.Request) {
	s.handleLifecycleEnqueue(w, r, "restart")
}

func (s *Server) handleDestroy(w http.ResponseWriter, r *http.Request) {
	s.handleLifecycleEnqueue(w, r, "destroy")
}

func (s *Server) handleGetJob(w http.ResponseWriter, r *http.Request) {
	job, err := s.store.GetDeploymentJob(r.Context(), chi.URLParam(r, "jobID"))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, r, http.StatusNotFound, "DEPLOYMENT_JOB_NOT_FOUND", "deployment job not found", nil)
			return
		}
		writeInternalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, domain.DeploymentJobResponse{Job: job})
}

func (s *Server) handleListJobs(w http.ResponseWriter, r *http.Request) {
	page, pageSize := parsePagination(r)
	jobs, err := s.store.ListDeploymentJobs(r.Context(), domain.DeploymentJobFilter{
		TenantID: r.URL.Query().Get("tenant_id"),
		JobType:  r.URL.Query().Get("job_type"),
		Status:   r.URL.Query().Get("status"),
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		writeInternalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, domain.DeploymentJobsResponse{Jobs: jobs})
}

func (s *Server) handleGetInstance(w http.ResponseWriter, r *http.Request) {
	instance, err := s.store.GetCurrentTenantInstance(r.Context(), chi.URLParam(r, "tenantID"))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, r, http.StatusNotFound, "INSTANCE_NOT_FOUND", "instance not found", nil)
			return
		}
		writeInternalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, domain.TenantInstanceResponse{Instance: instance})
}

func (s *Server) handleListInstances(w http.ResponseWriter, r *http.Request) {
	instances, err := s.store.ListTenantInstances(r.Context(), chi.URLParam(r, "tenantID"))
	if err != nil {
		writeInternalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, domain.TenantInstancesResponse{Instances: instances})
}

func (s *Server) handleListProviders(w http.ResponseWriter, r *http.Request) {
	providers, err := s.store.ListLLMProviders(r.Context())
	if err != nil {
		writeInternalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, domain.LLMProvidersResponse{Providers: providers})
}

func (s *Server) handleUpsertProvider(w http.ResponseWriter, r *http.Request) {
	var input domain.UpsertLLMProviderRequest
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", err.Error(), nil)
		return
	}
	if input.Name == "" {
		writeError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "name is required", nil)
		return
	}
	provider, err := s.store.UpsertLLMProvider(r.Context(), input)
	if err != nil {
		writeInternalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, domain.LLMProviderResponse{Provider: provider})
}

func (s *Server) handleGetProvider(w http.ResponseWriter, r *http.Request) {
	provider, err := s.store.GetLLMProvider(r.Context(), chi.URLParam(r, "providerID"))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, r, http.StatusNotFound, "PROVIDER_NOT_FOUND", "provider not found", nil)
			return
		}
		writeInternalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, domain.LLMProviderResponse{Provider: provider})
}

func (s *Server) handleDeleteProvider(w http.ResponseWriter, r *http.Request) {
	if err := s.store.DeleteLLMProvider(r.Context(), chi.URLParam(r, "providerID")); err != nil {
		writeInternalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (s *Server) handleListAPIKeys(w http.ResponseWriter, r *http.Request) {
	providerID := r.URL.Query().Get("provider_id")
	keys, err := s.store.ListLLMAPIKeys(r.Context(), providerID)
	if err != nil {
		writeInternalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, domain.LLMAPIKeysResponse{APIKeys: keys})
}

func (s *Server) handleAddAPIKey(w http.ResponseWriter, r *http.Request) {
	var input domain.AddLLMAPIKeyRequest
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", err.Error(), nil)
		return
	}
	if input.ProviderID == "" || input.Value == "" {
		writeError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "provider_id and value are required", nil)
		return
	}
	apiKey, err := s.store.AddLLMAPIKey(r.Context(), input.ProviderID, input.Value, input.Status)
	if err != nil {
		writeInternalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, domain.LLMAPIKeyResponse{APIKey: apiKey})
}

func (s *Server) handleGetAPIKey(w http.ResponseWriter, r *http.Request) {
	apiKey, err := s.store.GetLLMAPIKey(r.Context(), chi.URLParam(r, "keyID"))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, r, http.StatusNotFound, "API_KEY_NOT_FOUND", "api key not found", nil)
			return
		}
		writeInternalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, domain.LLMAPIKeyResponse{APIKey: apiKey})
}

func (s *Server) handleDeactivateAPIKey(w http.ResponseWriter, r *http.Request) {
	if err := s.store.DeactivateLLMAPIKey(r.Context(), chi.URLParam(r, "keyID")); err != nil {
		writeInternalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deactivated"})
}

func (s *Server) handleGetTenantLLMAllocation(w http.ResponseWriter, r *http.Request) {
	alloc, err := s.store.GetTenantLLMAllocation(r.Context(), chi.URLParam(r, "tenantID"))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, r, http.StatusNotFound, "ALLOCATION_NOT_FOUND", "no LLM key allocated to this tenant", nil)
			return
		}
		writeInternalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, domain.TenantLLMAllocationResponse{Allocation: alloc})
}

func (s *Server) handleAllocateLLMKey(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	var input domain.AllocateLLMKeyRequest
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", err.Error(), nil)
		return
	}
	if input.APIKeyID == "" || input.ModelName == "" {
		writeError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "api_key_id and model_name are required", nil)
		return
	}
	alloc, err := s.store.AllocateLLMKeyToTenant(r.Context(), tenantID, input.APIKeyID, input.ModelName)
	if err != nil {
		writeInternalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, domain.TenantLLMAllocationResponse{Allocation: alloc})
}

func (s *Server) handleRevokeLLMAllocation(w http.ResponseWriter, r *http.Request) {
	if err := s.store.RevokeTenantLLMAllocation(r.Context(), chi.URLParam(r, "tenantID")); err != nil {
		writeInternalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "revoked"})
}

func (s *Server) handleLifecycleEnqueue(w http.ResponseWriter, r *http.Request, jobType string) {
	tenantID := chi.URLParam(r, "tenantID")
	if _, err := s.store.GetTenant(r.Context(), tenantID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, r, http.StatusNotFound, "TENANT_NOT_FOUND", "tenant not found", nil)
			return
		}
		writeInternalError(w, r, err)
		return
	}

	action, err := decodeDeploymentAction(r)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", err.Error(), nil)
		return
	}
	if activeJob, err := s.store.FindActiveJob(r.Context(), tenantID); err == nil {
		writeJSON(w, http.StatusConflict, map[string]any{
			"error": map[string]any{
				"code":       "JOB_ALREADY_RUNNING",
				"message":    "another lifecycle job is already running for this tenant",
				"details":    map[string]any{"job_id": activeJob.ID},
				"request_id": middleware.GetReqID(r.Context()),
			},
		})
		return
	} else if !errors.Is(err, pgx.ErrNoRows) {
		writeInternalError(w, r, err)
		return
	}

	if jobType == "deploy" || jobType == "redeploy" {
		profile, err := s.store.GetTenantProfile(r.Context(), tenantID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				writeError(w, r, http.StatusNotFound, "TENANT_PROFILE_INVALID", "tenant profile not found", nil)
				return
			}
			writeInternalError(w, r, err)
			return
		}
		if !profile.Validation.IsValid {
			writeError(w, r, http.StatusBadRequest, "TENANT_PROFILE_INVALID", "tenant profile is invalid", map[string]any{"errors": profile.Validation.Errors})
			return
		}
		if _, err := s.store.ResolveImageForTemplate(r.Context(), profile.TemplateID); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				writeError(w, r, http.StatusBadRequest, "IMAGE_NOT_AVAILABLE", "no active image available for the tenant profile", nil)
				return
			}
			writeInternalError(w, r, err)
			return
		}
	}

	if jobType == "stop" || jobType == "start" || jobType == "restart" || jobType == "destroy" {
		if _, err := s.store.GetCurrentTenantInstance(r.Context(), tenantID); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				writeError(w, r, http.StatusNotFound, "INSTANCE_NOT_FOUND", "instance not found", nil)
				return
			}
			writeInternalError(w, r, err)
			return
		}
	}

	payload, err := json.Marshal(action)
	if err != nil {
		writeInternalError(w, r, err)
		return
	}
	job, err := s.store.EnqueueDeploymentJob(r.Context(), tenantID, jobType, "api", r.Header.Get("Idempotency-Key"), payload)
	if err != nil {
		writeInternalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, domain.DeploymentJobResponse{Job: job})
}

func parsePagination(r *http.Request) (int, int) {
	page := parseIntOrDefault(r.URL.Query().Get("page"), 1)
	pageSize := parseIntOrDefault(r.URL.Query().Get("page_size"), 20)
	return page, pageSize
}

func parseIntOrDefault(raw string, fallback int) int {
	if strings.TrimSpace(raw) == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}

func decodeJSON(r *http.Request, target any) error {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("decode json body: %w", err)
	}
	if decoder.More() {
		return fmt.Errorf("request body must contain a single JSON object")
	}
	return nil
}

func decodeDeploymentAction(r *http.Request) (domain.DeploymentActionRequest, error) {
	var action domain.DeploymentActionRequest
	if r.Body == nil {
		return action, nil
	}
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&action); err != nil {
		if errors.Is(err, io.EOF) {
			return action, nil
		}
		return action, fmt.Errorf("decode json body: %w", err)
	}
	if decoder.More() {
		return action, fmt.Errorf("request body must contain a single JSON object")
	}
	return action, nil
}

func writeJSON(w http.ResponseWriter, statusCode int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, r *http.Request, statusCode int, code string, message string, details any) {
	writeJSON(w, statusCode, map[string]any{
		"error": map[string]any{
			"code":       code,
			"message":    message,
			"details":    details,
			"request_id": middleware.GetReqID(r.Context()),
		},
	})
}

func writeInternalError(w http.ResponseWriter, r *http.Request, err error) {
	writeError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
}

func WithTimeout(parent context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parent, timeout)
}
