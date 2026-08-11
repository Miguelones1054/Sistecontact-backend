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
	"github.com/sistecontact/api/internal/contactstatus"
	"github.com/sistecontact/api/internal/fireapp"
	"github.com/sistecontact/api/internal/googlemaps"
	"github.com/sistecontact/api/internal/httpserver"
	"github.com/sistecontact/api/internal/prospects"
	"github.com/sistecontact/api/internal/search"
	"github.com/sistecontact/api/internal/visits"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	cfg, err := config.Load()
	if err != nil {
		logger.Error("config inválida", "err", err)
		os.Exit(1)
	}

	ctx := context.Background()
	fb, err := fireapp.New(ctx, cfg.FirebaseCredentialsFile)
	if err != nil {
		logger.Error("firebase", "err", err)
		os.Exit(1)
	}
	defer fb.Close()

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
	visitStore := visits.NewStore(fb.Firestore)
	prospectStore := prospects.NewStore(fb.Firestore)
	contactStore := contactstatus.NewStore(fb.Firestore)
	srv := httpserver.New(":"+cfg.Port, svc, visitStore, prospectStore, contactStore, fb.Auth, logger)

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

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("apagado", "err", err)
	}
	logger.Info("servidor detenido")
}
