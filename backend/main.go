package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/Mario-Miguel/folixenda/backend/auth"
	"github.com/Mario-Miguel/folixenda/backend/database"
	"github.com/Mario-Miguel/folixenda/backend/geocode"
	"github.com/Mario-Miguel/folixenda/backend/handlers"
	"github.com/Mario-Miguel/folixenda/backend/middleware"
	"github.com/Mario-Miguel/folixenda/backend/migrations"
	"github.com/Mario-Miguel/folixenda/backend/storage"
	"github.com/Mario-Miguel/folixenda/backend/store"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	//INIT DB
	database.Init(".env")

	// Apply pending schema migrations (backend/migrations) before serving.
	migrated, err := migrations.Up(context.Background(), database.DB)
	if err != nil {
		logger.Error("failed to apply migrations", "err", err)
		os.Exit(1)
	}
	for _, m := range migrated {
		logger.Info("migration applied", "version", m.Source.Version, "file", m.Source.Path, "took", m.Duration)
	}

	// Storage for event images. Fail fast: uploads and image URLs depend on it.
	objects, uploadsHandler, err := initStorage(logger, port)
	if err != nil {
		logger.Error("object storage not available", "err", err)
		os.Exit(1)
	}

	eventStore := store.NewEventStore(database.DB, objects.URL)
	userStore := store.NewPostgresUserStore(database.DB)
	savedEventsStore := store.NewUserSavedEventsStore(database.DB, objects.URL)
	publisherProfileStore := store.NewPublisherProfileStore(database.DB)
	notificationPrefsStore := store.NewNotificationPreferencesStore(database.DB)

	// FINISHED CREATING DB

	// CREATE MIDDLEWARES: AUTH, ETC
	appCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	verifier, err := auth.NewVerifier(appCtx,
		os.Getenv("AUTH_JWKS_URL"),
		os.Getenv("AUTH_ISSUER"),
		os.Getenv("AUTH_AUDIENCE"),
	)
	if err != nil {
		logger.Error("failed to init token verifier", "err", err)
		os.Exit(1)
	}

	authenticate := middleware.Authenticate(verifier)
	provision := middleware.Provision(userStore)

	// Compone ambos: primero autentica, luego provisiona
	protect := func(h http.Handler) http.Handler {
		return authenticate(provision(h))
	}

	eventsHandler := handlers.NewEventsHandler(eventStore, eventStore, objects)
	geocodeHandler := handlers.NewGeocodeHandler(geocode.New(geocode.Options{}))
	usersHandler := handlers.NewUsersHandler(userStore)
	savedEventsHandler := handlers.NewSavedEventsHandler(savedEventsStore)
	meHandler := handlers.NewMeHandler(userStore, publisherProfileStore, notificationPrefsStore, eventStore)

	mux := http.NewServeMux()
	eventsHandler.Register(mux, protect)
	usersHandler.Register(mux, protect)
	savedEventsHandler.Register(mux, protect)
	meHandler.Register(mux, protect)
	geocodeHandler.Register(mux, protect)

	// Health check
	// Local storage serves uploaded files itself; S3-compatible storage serves them directly.
	if uploadsHandler != nil {
		mux.Handle("GET /uploads/", http.StripPrefix("/uploads", uploadsHandler))
	}

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "ok")
	})

	handler := middleware.Cors(middleware.Logging(logger, mux))

	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      handler,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		logger.Info("server starting", "port", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server error", "err", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("shutdown error", "err", err)
	}
	logger.Info("server stopped")
}

// initStorage picks where event images live (STORAGE_DRIVER):
//   - "local" (default): files under LOCAL_STORAGE_DIR (default ./uploads),
//     served by this API at /uploads (LOCAL_STORAGE_PUBLIC_URL, default
//     http://localhost:PORT/uploads). For development.
//   - "s3": any S3-compatible service (AWS S3, Cloudflare R2, MinIO...)
//     configured with the S3_* variables.
//
// The returned handler is non-nil only for local storage and must be mounted
// at /uploads.
func initStorage(logger *slog.Logger, port string) (storage.Storage, http.Handler, error) {
	switch driver := strings.ToLower(strings.TrimSpace(os.Getenv("STORAGE_DRIVER"))); driver {
	case "", "local":
		dir := envOr("LOCAL_STORAGE_DIR", "uploads")
		publicURL := envOr("LOCAL_STORAGE_PUBLIC_URL", "http://localhost:"+port+"/uploads")
		l, err := storage.NewLocal(dir, publicURL)
		if err != nil {
			return nil, nil, err
		}
		logger.Info("object storage ready", "driver", "local", "dir", l.Dir(), "url", publicURL)
		return l, l.Handler(), nil
	case "s3":
		cfg, err := storage.ConfigFromEnv()
		if err != nil {
			return nil, nil, err
		}
		m, err := storage.NewMinIO(cfg)
		if err != nil {
			return nil, nil, err
		}
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := m.EnsureBucket(ctx); err != nil {
			return nil, nil, err
		}
		logger.Info("object storage ready", "driver", "s3", "endpoint", cfg.Endpoint, "bucket", cfg.Bucket)
		return m, nil, nil
	default:
		return nil, nil, fmt.Errorf("STORAGE_DRIVER=%q: expected local or s3", driver)
	}
}

func envOr(name, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(name)); v != "" {
		return v
	}
	return fallback
}
