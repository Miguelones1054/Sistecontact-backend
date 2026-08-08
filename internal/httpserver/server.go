package httpserver

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/sistecontact/api/internal/search"
)

type Server struct {
	httpSrv *http.Server
	logger  *slog.Logger
}

func New(addr string, svc *search.Service, logger *slog.Logger) *Server {
	h := NewHandler(svc)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", h.health)
	mux.HandleFunc("GET /api/zones", h.zones)
	mux.HandleFunc("GET /api/search", h.search)

	stack := chain(mux, cors, recoverPanic(logger), logging(logger))

	return &Server{
		httpSrv: &http.Server{
			Addr:              addr,
			Handler:           stack,
			ReadHeaderTimeout: 5 * time.Second,
			ReadTimeout:       15 * time.Second,
			// Búsquedas con subdivisión de zona pueden tardar varios minutos.
			WriteTimeout: 5 * time.Minute,
			IdleTimeout:  120 * time.Second,
		},
		logger: logger,
	}
}

func (s *Server) Start() error {
	s.logger.Info("servidor escuchando", "addr", s.httpSrv.Addr)
	return s.httpSrv.ListenAndServe()
}

// Shutdown detiene el servidor de forma ordenada esperando conexiones activas.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpSrv.Shutdown(ctx)
}

type middleware func(http.Handler) http.Handler

func chain(h http.Handler, mws ...middleware) http.Handler {
	for i := len(mws) - 1; i >= 0; i-- {
		h = mws[i](h)
	}
	return h
}
