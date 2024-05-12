package main

import (
	"context"
	"do-short-url/internal/config"
	"do-short-url/internal/http-server/handlers/redirect"
	"do-short-url/internal/http-server/handlers/url/save"
	mwLogger "do-short-url/internal/http-server/middleware/logger"
	"do-short-url/internal/lib/logger/sl"
	"do-short-url/internal/storage/sqlite"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"log/slog"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func main() {

	cfg := config.MustLoad()

	log := setupLogger(cfg.Env)
	log.Info("starting do-short-url", slog.String("env", cfg.Env))
	log.Debug("debug message are enabled")

	storage, err := sqlite.New(cfg.StoragePath)
	if err != nil {
		log.Error("failed to init storage", sl.Err(err))
		os.Exit(1)
	}

	router := chi.NewRouter()
	router.Use(middleware.RequestID)
	router.Use(mwLogger.New(log))
	router.Use(middleware.Recoverer)
	router.Use(middleware.URLFormat)

	router.Route("/url", func(r chi.Router) {
		// curl -X POST localhost:8888/url -H "Content-Type: application/json" -H "Authorization: Basic $(echo -n "myuser:mypass" | base64)" -d '{"URL": "https://linkedin.com"}'
		r.Use(middleware.BasicAuth("do-short-url", map[string]string{
			cfg.HTTPServer.User: cfg.HTTPServer.Password,
		}))
		r.Post("/", save.New(log, storage))
		// r.Delete("/{alias}", delete.New(log, storage)) // TODO
	})

	router.Get("/{alias}", redirect.New(log, storage))

	log.Info("starting server", slog.String("address", cfg.Address))

	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	srv := &http.Server{
		Addr:         cfg.Address,
		Handler:      router,
		ReadTimeout:  cfg.HTTPServer.Timeout,
		WriteTimeout: cfg.HTTPServer.Timeout,
		IdleTimeout:  cfg.HTTPServer.IdleTimeout,
	}

	go func() {
		err := srv.ListenAndServe()
		if err != nil {
			if err == http.ErrServerClosed {
				log.Info("server stopped")
			} else {
				log.Error("failed to start server")
			}
		}
	}()

	log.Info("server started")

	<-done
	log.Info("stopping server")

	// TODO: move timeout to config
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Error("failed to stop server", sl.Err(err))
		return
	}

	// TODO: close storage
}

const (
	envLocal = "local"
	envProd  = "prod"
)

func setupLogger(env string) *slog.Logger {
	var log *slog.Logger
	switch env {
	case envLocal:
		log = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	case envProd:
		log = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	}
	return log
}
