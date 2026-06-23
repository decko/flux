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

	_ "github.com/mattn/go-sqlite3"

	"github.com/decko/flux/internal/adapter/orchestrator"
	"github.com/decko/flux/internal/adapter/scm"
	"github.com/decko/flux/internal/adapter/ticket"
	"github.com/decko/flux/internal/api"
	"github.com/decko/flux/internal/config"
	"github.com/decko/flux/internal/domain"
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
	db, err := sql.Open("sqlite3", cfg.Database.Path)
	if err != nil {
		return nil, nil, fmt.Errorf("open database: %w", err)
	}

	if err := repository.ConfigureSQLiteDB(db); err != nil {
		_ = db.Close()
		return nil, nil, fmt.Errorf("configure database: %w", err)
	}

	projectRepo := repository.NewSQLiteProjectRepository(db)
	ticketRepo := repository.NewSQLiteTicketRepository(db)
	prRepo := repository.NewSQLitePullRequestRepository(db)
	pipelineRepo := repository.NewSQLitePipelineRunRepository(db)
	userRepo := repository.NewSQLiteUserRepository(db)

	if err := projectRepo.Migrate(ctx); err != nil {
		_ = db.Close()
		return nil, nil, fmt.Errorf("migrate projects: %w", err)
	}
	if err := ticketRepo.Migrate(ctx); err != nil {
		_ = db.Close()
		return nil, nil, fmt.Errorf("migrate tickets: %w", err)
	}
	if err := prRepo.Migrate(ctx); err != nil {
		_ = db.Close()
		return nil, nil, fmt.Errorf("migrate pull requests: %w", err)
	}
	if err := pipelineRepo.Migrate(ctx); err != nil {
		_ = db.Close()
		return nil, nil, fmt.Errorf("migrate pipeline runs: %w", err)
	}
	if err := userRepo.Migrate(ctx); err != nil {
		_ = db.Close()
		return nil, nil, fmt.Errorf("migrate users: %w", err)
	}

	projectSvc := domain.NewProjectService(projectRepo)
	ticketSvc := domain.NewTicketService(ticketRepo)
	prSvc := domain.NewPullRequestService(prRepo)
	pipelineSvc := domain.NewPipelineRunService(pipelineRepo)

	// Wire soda orchestrator if configured.
	for _, o := range cfg.Orchestrators {
		if o.Type == "soda" {
			slog.Info("configuring soda orchestrator", "path", o.Path)
			pipelineSvc = domain.NewPipelineRunService(pipelineRepo,
				domain.WithOrchestrator(orchestrator.NewSodaAdapter(o.Path)))
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
	syncSvc := domain.NewSyncService(ticketRepo, prRepo, nil, nil, syncInterval)

	// Build GitHub adapters if a token is configured.
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		for _, a := range cfg.Adapters {
			if a.Type == "github" {
				slog.Info("configuring github adapter", "owner", a.Owner, "repo", a.Repo)
				ticketAdapter := ticket.NewGitHubAdapter(a.Owner, a.Repo, token, nil)
				scmAdapter := scm.NewGitHubAdapter(a.Owner, a.Repo, token, nil)
				syncSvc.TicketAdapter = ticketAdapter
				syncSvc.SCMAdapter = scmAdapter
				break // first github adapter only for now
			}
		}
	}

	// Start background sync loop.
	go syncSvc.Run(ctx)

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
