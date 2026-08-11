package httpserver

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"firebase.google.com/go/v4/auth"
	"github.com/sistecontact/api/internal/contactstatus"
	"github.com/sistecontact/api/internal/prospects"
	"github.com/sistecontact/api/internal/search"
	"github.com/sistecontact/api/internal/visits"
)

type Server struct {
	httpSrv *http.Server
	logger  *slog.Logger
}

func New(
	addr string,
	svc *search.Service,
	visitStore *visits.Store,
	prospectStore *prospects.Store,
	contactStore *contactstatus.Store,
	authClient *auth.Client,
	logger *slog.Logger,
) *Server {
	h := NewHandler(svc, visitStore, prospectStore, contactStore)
	authMW := requireAuth(authClient)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", h.health)
	mux.HandleFunc("GET /api/zones", h.zones)
	mux.HandleFunc("GET /api/search", h.search)

	mux.Handle("GET /api/visits", authMW(http.HandlerFunc(h.listVisits)))
	mux.Handle("PUT /api/visits/{placeId}", authMW(http.HandlerFunc(h.upsertVisit)))
	mux.Handle("DELETE /api/visits/{placeId}", authMW(http.HandlerFunc(h.deleteVisit)))
	mux.Handle("GET /api/businesses/{placeId}/visitors", authMW(http.HandlerFunc(h.listBusinessVisitors)))
	mux.Handle("GET /api/businesses/{placeId}/scheduled", authMW(http.HandlerFunc(h.listBusinessScheduled)))

	mux.Handle("GET /api/prospects", authMW(http.HandlerFunc(h.listProspects)))
	mux.Handle("PUT /api/prospects/{placeId}", authMW(http.HandlerFunc(h.upsertProspect)))
	mux.Handle("DELETE /api/prospects/{placeId}", authMW(http.HandlerFunc(h.deleteProspect)))

	mux.Handle("GET /api/contact-status", authMW(http.HandlerFunc(h.listContactStatus)))
	mux.Handle("PUT /api/contact-status/{placeId}", authMW(http.HandlerFunc(h.upsertContactStatus)))

	stack := chain(mux, cors, recoverPanic(logger), logging(logger))

	return &Server{
		httpSrv: &http.Server{
			Addr:              addr,
			Handler:           stack,
			ReadHeaderTimeout: 5 * time.Second,
			ReadTimeout:       15 * time.Second,
			WriteTimeout:      5 * time.Minute,
			IdleTimeout:       120 * time.Second,
		},
		logger: logger,
	}
}

func (s *Server) Start() error {
	s.logger.Info("servidor escuchando", "addr", s.httpSrv.Addr)
	return s.httpSrv.ListenAndServe()
}

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
