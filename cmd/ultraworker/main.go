package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"openclaw-autodeploy/internal/config"
	"openclaw-autodeploy/internal/db"
	internaldocker "openclaw-autodeploy/internal/docker"
	"openclaw-autodeploy/internal/jobs"
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
	interval, err := cfg.WorkerPollInterval()
	if err != nil {
		log.Fatalf("parse worker poll interval: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := db.Open(ctx, cfg.Database.URL)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer pool.Close()

	store := storepkg.New(pool, cfg.Security.MasterKey)
	runtime, err := internaldocker.New()
	if err != nil {
		log.Fatalf("create docker client: %v", err)
	}
	executor, err := jobs.NewExecutor(cfg, store, runtime)
	if err != nil {
		log.Fatalf("create jobs executor: %v", err)
	}
	if err := store.UpsertWorkerHeartbeat(ctx, cfg.Worker.Name); err != nil {
		log.Fatalf("write initial heartbeat: %v", err)
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	log.Printf("ultraworker heartbeat loop started for %s", cfg.Worker.Name)
	for {
		select {
		case <-ctx.Done():
			log.Printf("ultraworker stopping")
			return
		case <-ticker.C:
			if err := store.UpsertWorkerHeartbeat(ctx, cfg.Worker.Name); err != nil {
				log.Printf("write heartbeat: %v", err)
			}
			for {
				processed, err := executor.ProcessOnce(ctx)
				if err != nil {
					log.Printf("process job: %v", err)
					break
				}
				if !processed {
					break
				}
			}
		}
	}
}
