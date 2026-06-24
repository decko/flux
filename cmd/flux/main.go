package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jmoiron/sqlx"
	_ "modernc.org/sqlite"

	"github.com/decko/flux/internal/adapter/orchestrator"
	"github.com/decko/flux/internal/adapter/scm"
	"github.com/decko/flux/internal/adapter/ticket"
	"github.com/decko/flux/internal/api"
	"github.com/decko/flux/internal/config"
	"github.com/decko/flux/internal/domain"
	dbMigration "github.com/decko/flux/internal/migration"
	"github.com/decko/flux/internal/repository"
)

func main() {
	if err := run(); err != nil {
		slog.Error("flux", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load("flux.yaml")
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("validate config: %w", err)
	}

	setupLogging(cfg.Logging.Level)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	srv, cleanup, err := setupServer(ctx, cfg)
	if err != nil {
		return fmt.Errorf("setup server: %w", err)
	}
	defer cleanup()

	httpServer := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:      srv,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		slog.Info("flux listening", "addr", httpServer.Addr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("http server error", "error", err)
		}
	}()

	<-ctx.Done()
	slog.Info("shutting down gracefully")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}

	return nil
}

// jwtSecret returns the JWT signing key from the JWT_SECRET environment
// variable, or a development fallback if not set.
// It terminates the process if the secret is shorter than 16 characters,
// as short secrets are a security risk. The dev fallback "dev-secret"
// is intentionally short to fail closed in production; set JWT_SECRET
// to a value of at least 16 characters.
// Tests may set NO_AUTH=1 to bypass the check.
func jwtSecret() []byte {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		secret = "dev-secret"
	}
	if len(secret) < 16 && os.Getenv("NO_AUTH") != "1" {
		log.Fatalf("JWT_SECRET must be at least 16 characters (got %d)", len(secret))
	}
	return []byte(secret)
}

// setupLogging configures the default slog logger with the given level.
func setupLogging(level string) {
	var slogLevel slog.Level
	switch level {
	case "debug":
		slogLevel = slog.LevelDebug
	case "warn":
		slogLevel = slog.LevelWarn
	case "error":
		slogLevel = slog.LevelError
	default:
		slogLevel = slog.LevelInfo
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slogLevel})))
}

// setupServer wires together all dependencies — database, repositories,
// services, and the API server — returning the server, a cleanup function
// that closes the database, and any error encountered during setup.
func setupServer(ctx context.Context, cfg *config.Config) (*api.Server, func(), error) {
	db, err := sql.Open("sqlite", cfg.Database.Path)
	if err != nil {
		return nil, nil, fmt.Errorf("open database: %w", err)
	}

	if err := repository.ConfigureSQLiteDB(db); err != nil {
		_ = db.Close()
		return nil, nil, fmt.Errorf("configure database: %w", err)
	}

	// Run database migrations.
	if err := dbMigration.Up(db); err != nil {
		_ = db.Close()
		return nil, nil, fmt.Errorf("run migrations: %w", err)
	}

	sdb := sqlx.NewDb(db, "sqlite")

	projectRepo := repository.NewSQLiteProjectRepository(sdb)
	ticketRepo := repository.NewSQLiteTicketRepository(sdb)
	prRepo := repository.NewSQLitePullRequestRepository(sdb)
	pipelineRepo := repository.NewSQLitePipelineRunRepository(sdb)
	userRepo := repository.NewSQLiteUserRepository(sdb)
	auditRepo := repository.NewSQLiteAuditRepository(sdb)
	auditSvc := domain.NewAuditService(auditRepo)

	projectSvc := domain.NewProjectService(projectRepo)
	ticketSvc := domain.NewTicketService(ticketRepo)
	prSvc := domain.NewPullRequestService(prRepo)
	pipelineSvc := domain.NewPipelineRunService(pipelineRepo)

	// Wire soda orchestrator if configured.
	for _, o := range cfg.Orchestrators {
		if o.Type == "soda" {
			slog.Info("configuring soda orchestrator", "path", o.Path)
			pipelineSvc = domain.NewPipelineRunService(pipelineRepo,
				domain.WithOrchestrator(orchestrator.NewSodaAdapter(o.Path,
					orchestrator.WithSodaConfig("soda.yaml"))))
			break
		}
	}
	jwtSecret := jwtSecret()
	authSvc := domain.NewAuthService(userRepo, jwtSecret)

	syncInterval, err := time.ParseDuration(cfg.Sync.Interval)
	if err != nil {
		_ = db.Close()
		return nil, nil, fmt.Errorf("parse sync interval: %w", err)
	}

	// Build factory for per-project adapters.
	factory := func(projectID string) (ticket.TicketAdapter, scm.SCMAdapter, error) {
		project, err := projectRepo.Get(ctx, projectID)
		if err != nil {
			return nil, nil, fmt.Errorf("get project %s: %w", projectID, err)
		}
		ghToken := os.Getenv("GITHUB_TOKEN")
		for _, a := range project.Adapters {
			if a.Type == "github" {
				owner := a.Config["owner"]
				repo := a.Config["repo"]
				if owner == "" || repo == "" {
					return nil, nil, fmt.Errorf("github adapter missing owner or repo")
				}
				slog.Info("configuring github adapter", "project_id", projectID, "owner", owner, "repo", repo)
				return ticket.NewGitHubAdapter(owner, repo, ghToken, nil),
					scm.NewGitHubAdapter(owner, repo, ghToken, nil),
					nil
			}
		}
		// Fallback: use project's installation_id with GitHub App auth.
		if ghToken != "" {
			slog.Warn("using GITHUB_TOKEN fallback for project", "project_id", projectID)
			return ticket.NewGitHubAdapter("unknown", "unknown", ghToken, nil),
				scm.NewGitHubAdapter("unknown", "unknown", ghToken, nil),
				nil
		}
		return nil, nil, fmt.Errorf("no adapters available for project %s", projectID)
	}
	syncSvc := domain.NewSyncService(ticketRepo, prRepo, projectRepo, factory, syncInterval)

	// Start background sync loop.
	go syncSvc.Run(ctx)

	// Start periodic audit cleanup goroutine.
	go func() {
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if err := auditSvc.PurgeOldEvents(ctx, cfg.Audit.RetentionDays); err != nil {
					slog.Error("audit cleanup", "error", err)
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	srv := api.NewServer(
		api.WithCORSOrigin(cfg.CORS.Origin),
		api.WithJWTSecret(jwtSecret),
		api.WithProjectService(projectSvc),
		api.WithTicketService(ticketSvc),
		api.WithPRService(prSvc),
		api.WithPipelineService(pipelineSvc),
		api.WithAuthService(authSvc),
		api.WithSyncService(syncSvc),
		api.WithAdapters(buildAdapterMap(cfg.Adapters)),
		api.WithAuditService(auditSvc),
		api.WithSPA(),
	)

	return srv, func() { _ = db.Close() }, nil
}

// buildAdapterMap converts a slice of config AdapterEntry to a map of
// adapter type to AdapterInfo, used for adapter listing and health checks.
func buildAdapterMap(entries []config.AdapterEntry) map[string]domain.AdapterInfo {
	if len(entries) == 0 {
		return nil
	}
	m := make(map[string]domain.AdapterInfo, len(entries))
	for _, e := range entries {
		// Use the first entry of each adapter type; duplicates are overwritten.
		m[e.Type] = domain.AdapterInfo{
			Type:   e.Type,
			Name:   e.Type, // fallback name is the type key
			Health: "unknown",
		}
	}
	return m
}
