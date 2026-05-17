package main

import (
	"context"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/r0lm0/go-saas-api/internal/agent/handler"
	"github.com/r0lm0/go-saas-api/internal/agent/memory"
	"github.com/r0lm0/go-saas-api/internal/agent/repository"
	"github.com/r0lm0/go-saas-api/internal/agent/runtime"
	"github.com/r0lm0/go-saas-api/internal/agent/store"
	"github.com/r0lm0/go-saas-api/internal/agent/tools"
	"github.com/r0lm0/go-saas-api/internal/agent/usage"
	"github.com/r0lm0/go-saas-api/internal/agent/websocket"
	"github.com/r0lm0/go-saas-api/internal/billing"
	"github.com/r0lm0/go-saas-api/internal/document"
	"github.com/r0lm0/go-saas-api/internal/fileupload"
	"github.com/r0lm0/go-saas-api/internal/platform/config"
	"github.com/r0lm0/go-saas-api/internal/platform/health"
	"github.com/r0lm0/go-saas-api/internal/platform/logger"
	"github.com/r0lm0/go-saas-api/internal/platform/middleware"
	"github.com/r0lm0/go-saas-api/internal/platform/nats"
	"github.com/r0lm0/go-saas-api/internal/platform/postgres"
	"github.com/r0lm0/go-saas-api/internal/platform/redis"
	"github.com/r0lm0/go-saas-api/internal/provider"
	"github.com/r0lm0/go-saas-api/pkg/jwt"
	"github.com/r0lm0/go-saas-api/pkg/llm"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// runMigrations applies automatic database migrations
func runMigrations(ctx context.Context, pool *pgxpool.Pool, log logger.Logger) error {
	log.Info("running auto-migrations")

	// Check if artifact_id column exists in messages table
	var exists bool
	err := pool.QueryRow(ctx,
		`SELECT EXISTS (
			SELECT 1 FROM information_schema.columns 
			WHERE table_name = 'messages' AND column_name = 'artifact_id'
		)`).Scan(&exists)
	if err != nil {
		return fmt.Errorf("check artifact_id column: %w", err)
	}

	if !exists {
		log.Info("adding artifact_id column to messages table")
		_, err = pool.Exec(ctx,
			`ALTER TABLE messages 
			 ADD COLUMN artifact_id UUID REFERENCES artifacts(id) ON DELETE SET NULL`)
		if err != nil {
			return fmt.Errorf("add artifact_id column: %w", err)
		}
		log.Info("artifact_id column added successfully")
	} else {
		log.Info("artifact_id column already exists")
	}

	return nil
}

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}
	log := logger.New(cfg.LogLevel)
	defer log.Sync()
	masterKey := cfg.MasterEncryptionKey
	if masterKey == "" {
		masterKey = "go-saas-api-dev-master-key-32!"
		log.Warn("MASTER_ENCRYPTION_KEY not set, using default development key. THIS MUST BE CHANGED IN PRODUCTION!")
	}
	log.Info("starting agent-service", logger.String("port", cfg.Port), logger.String("env", cfg.Env))
	ctx := context.Background()
	pgPool, err := postgres.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatal("failed to connect to postgres", logger.Error(err))
	}
	defer pgPool.Close()

	// Auto-migration: ensure artifact_id column exists in messages table
	if err := runMigrations(ctx, pgPool, log); err != nil {
		log.Warn("auto-migration failed", logger.Error(err))
	}

	redisClient, err := redis.NewClient(cfg.RedisURL)
	if err != nil {
		log.Fatal("failed to connect to redis", logger.Error(err))
	}
	defer redisClient.Close()
	nc, err := nats.NewConn(cfg.NATSURL)
	if err != nil {
		log.Fatal("failed to connect to nats", logger.Error(err))
	}
	defer nc.Close()
	multiClient := llm.NewMultiClient()
	providerStore := provider.NewPostgresStore(pgPool)
	providerLoader := provider.NewProviderLoader(providerStore, multiClient, masterKey)
	if err := provider.MigrateKeys(ctx, providerStore, masterKey, log); err != nil {
		log.Warn("failed to migrate provider keys", logger.Error(err))
	}
	if err := providerLoader.LoadAll(ctx); err != nil {
		log.Warn("failed to load providers from database", logger.Error(err))
	}
	if len(multiClient.ListProviders()) == 0 {
		log.Warn("CRITICAL: No providers loaded from database. Chat will NOT work. Please configure providers via the admin API or database.")
	}
	multiClient.StartHealthChecks(ctx, 30*time.Second)
	llmClient := llm.Client(multiClient)
	providerService := provider.NewService(providerStore, masterKey)
	providerHandler := provider.NewHandler(providerService, log)
	convStore := store.NewConversationStore(pgPool)
	msgStore := store.NewMessageStore(pgPool)
	artStore := store.NewArtifactStore(pgPool)
	artFileStore := repository.NewArtifactFileRepo(pgPool)
	agentStore := store.NewAgentStore(pgPool)
	userCtxStore := store.NewUserContextStore(pgPool)
	toolRegistry := tools.NewRegistry()
	_ = toolRegistry.Register(tools.NewFileWriteTool(artStore))
	_ = toolRegistry.Register(tools.NewReadFileTool(artStore))
	_ = toolRegistry.Register(tools.NewCodeExecuteTool(tools.NewHTTPSandboxClient(cfg.SandboxServiceURL)))
	_ = toolRegistry.Register(tools.NewWebSearchTool())
	_ = toolRegistry.Register(tools.NewWebReaderTool())
	sessions := runtime.NewSessionManager(convStore, msgStore)
	fileStore := fileupload.NewPostgresStore(pgPool)
	fileService := fileupload.NewService(fileStore, cfg.UploadPath, cfg.MaxUploadSize)
	fileHandler := fileupload.NewHandler(fileService, log)
	var docClient *document.Client
	if cfg.DocumentServiceURL != "" {
		docClient = document.NewClient(cfg.DocumentServiceURL)
	}
	memoryExtractor := memory.NewExtractor(llmClient, userCtxStore, log)
	billingStore := billing.NewPostgresBillingStore(pgPool)
	usageTracker := usage.NewTracker(billingStore)
	orchestrator := runtime.NewOrchestrator(llmClient, toolRegistry, sessions, agentStore, artStore, artFileStore, fileService, docClient, providerStore, memoryExtractor, usageTracker, log)
	wsManager := websocket.NewManager(orchestrator, log)
	if cfg.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}
	hc := health.NewChecker(pgPool, redisClient, nc)
	jwtMgr := jwt.NewManager(cfg.JWTSecret)
	r := gin.New()
	r.Use(gin.Recovery(), middleware.RequestID(), middleware.Logger(log), middleware.CORS())
	agentHandler := handler.NewHandler(orchestrator, convStore, msgStore, artStore, userCtxStore, log, multiClient, providerStore, wsManager, hc, pgPool)
	agentHandler.RegisterRoutes(r)
	fileHandler.RegisterRoutes(r)
	providerHandler.RegisterRoutes(r, middleware.JWTAuth(jwtMgr))
	// Timeout de 5 minutos para todos los endpoints (incluyendo streaming SSE)
	// El LLM puede tardar hasta 3 minutos en generar una landing page
	r.Use(middleware.RequestTimeout(300 * time.Second))
	httpSrv := &http.Server{Addr: ":" + cfg.Port, Handler: r}
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("server failed", logger.Error(err))
		}
	}()
	log.Info("agent-service running", logger.String("addr", httpSrv.Addr))
	<-quit
	log.Info("shutting down agent-service")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := httpSrv.Shutdown(shutdownCtx); err != nil {
		log.Error("server forced to shutdown", logger.Error(err))
	}
	log.Info("agent-service exited")
}
