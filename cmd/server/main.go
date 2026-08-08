package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sistecontact/api/internal/config"
	"github.com/sistecontact/api/internal/googlemaps"
	"github.com/sistecontact/api/internal/httpserver"
	"github.com/sistecontact/api/internal/search"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	cfg, err := config.Load()
	if err != nil {
		logger.Error("config inválida", "err", err)
		os.Exit(1)
	}

	gmaps := googlemaps.New(cfg.GoogleMapsAPIKey, cfg.GoogleMapsLanguage, cfg.GoogleMapsRegion, cfg.GoogleHTTPTimeout)
	svc := search.New(
		gmaps,
		cfg.CacheTTL,
		cfg.CacheTTL,
		cfg.CacheCleanup,
		cfg.MaxPages,
		cfg.GridMaxDepth,
		cfg.SearchWorkers,
		logger,
	)
	srv := httpserver.New(":"+cfg.Port, svc, logger)

	// Captura señales para apagado ordenado.
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		if err := srv.Start(); err != nil && err != http.ErrServerClosed {
			logger.Error("servidor", "err", err)
			os.Exit(1)
		}
	}()

	sig := <-stop
	logger.Info("señal recibida, apagando", "signal", sig.String())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("apagado", "err", err)
	}
	logger.Info("servidor detenido")
}
