package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"openclaw-autodeploy/internal/api"
	"openclaw-autodeploy/internal/config"
	"openclaw-autodeploy/internal/db"
	internaldocker "openclaw-autodeploy/internal/docker"
	"openclaw-autodeploy/internal/service"
	storepkg "openclaw-autodeploy/internal/store/postgres"
)

func main() {
	var configPath string
	flag.StringVar(&configPath, "config", "", "Path to YAML config file")
	flag.Parse()

	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := db.Open(ctx, cfg.Database.URL)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer pool.Close()

	store := storepkg.New(pool, cfg.Security.MasterKey)
	validator := service.NewProfileValidator(store)
	dockerClient, err := internaldocker.New()
	if err != nil {
		log.Fatalf("create docker client: %v", err)
	}
	heartbeatTTL, err := cfg.WorkerHeartbeatTTL()
	if err != nil {
		log.Fatalf("parse heartbeat ttl: %v", err)
	}
	handler := api.New(store, validator, dockerClient, cfg.Worker.Name, heartbeatTTL, cfg.Security.StaticToken).Routes()

	server := &http.Server{
		Addr:              cfg.API.ListenAddr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Printf("shutdown server: %v", err)
		}
	}()

	log.Printf("control-plane-api listening on %s", cfg.API.ListenAddr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("listen and serve: %v", err)
	}
}
