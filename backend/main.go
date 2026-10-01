package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Mario-Miguel/folixenda/backend/auth"
	"github.com/Mario-Miguel/folixenda/backend/database"
	"github.com/Mario-Miguel/folixenda/backend/handlers"
	"github.com/Mario-Miguel/folixenda/backend/middleware"
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
	eventStore, err := store.NewEventStore(database.DB)
	if err != nil {
		logger.Error("failed to init event store", "err", err)
		os.Exit(1)
	}
	userStore, err := store.NewPostgresUserStore(database.DB)
	if err != nil {
		logger.Error("failed to init user store", "err", err)
		os.Exit(1)
	}

	savedEventsStore, err := store.NewUserSavedEventsStore(database.DB)
	if err != nil {
		logger.Error("failed to init user store", "err", err)
		os.Exit(1)
	}

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

	eventsHandler := handlers.NewEventsHandler(eventStore)
	usersHandler := handlers.NewUsersHandler(userStore)
	savedEventsHandler := handlers.NewSavedEventsHandler(savedEventsStore)

	mux := http.NewServeMux()
	eventsHandler.Register(mux, protect)
	usersHandler.Register(mux, protect)
	savedEventsHandler.Register(mux, protect)

	// Health check
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
