package httpserver

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"firebase.google.com/go/v4/auth"
	"github.com/sistecontact/api/internal/contactstatus"
	"github.com/sistecontact/api/internal/googlecalendar"
	"github.com/sistecontact/api/internal/prospects"
	"github.com/sistecontact/api/internal/search"
	"github.com/sistecontact/api/internal/usersettings"
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
	settingsStore *usersettings.Store,
	gcalStore *googlecalendar.Store,
	gcalOAuth *googlecalendar.OAuth,
	gcalClient *googlecalendar.Client,
	calendarTZ string,
	authClient *auth.Client,
	logger *slog.Logger,
) *Server {
	h := NewHandler(
		svc,
		visitStore,
		prospectStore,
		contactStore,
		settingsStore,
		gcalStore,
		gcalOAuth,
		gcalClient,
		calendarTZ,
		authClient,
	)
	authMW := requireAuth(authClient)
	accessMW := func(next http.Handler) http.Handler {
		return authMW(requireAccess(settingsStore)(next))
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", h.health)
	mux.HandleFunc("GET /api/zones", h.zones)
	mux.HandleFunc("GET /api/search", h.search)

	// Solo auth: crea el doc de acceso si falta y permite leer el flag (sin exigir true).
	mux.Handle("GET /api/settings/access", authMW(http.HandlerFunc(h.getAccessSettings)))

	mux.Handle("GET /api/visits", accessMW(http.HandlerFunc(h.listVisits)))
	mux.Handle("PUT /api/visits/{placeId}", accessMW(http.HandlerFunc(h.upsertVisit)))
	mux.Handle("DELETE /api/visits/{placeId}", accessMW(http.HandlerFunc(h.deleteVisit)))
	mux.Handle("GET /api/businesses/{placeId}/visitors", accessMW(http.HandlerFunc(h.listBusinessVisitors)))
	mux.Handle("GET /api/businesses/{placeId}/scheduled", accessMW(http.HandlerFunc(h.listBusinessScheduled)))

	mux.Handle("GET /api/prospects", accessMW(http.HandlerFunc(h.listProspects)))
	mux.Handle("PUT /api/prospects/{placeId}", accessMW(http.HandlerFunc(h.upsertProspect)))
	mux.Handle("DELETE /api/prospects/{placeId}", accessMW(http.HandlerFunc(h.deleteProspect)))

	mux.Handle("GET /api/contact-status", accessMW(http.HandlerFunc(h.listContactStatus)))
	mux.Handle("PUT /api/contact-status/{placeId}", accessMW(http.HandlerFunc(h.upsertContactStatus)))

	mux.Handle("GET /api/settings/scheduling", accessMW(http.HandlerFunc(h.getSchedulingSettings)))
	mux.Handle("PUT /api/settings/scheduling", accessMW(http.HandlerFunc(h.upsertSchedulingSettings)))

	mux.Handle("GET /api/integrations/google-calendar", accessMW(http.HandlerFunc(h.googleCalendarStatus)))
	mux.Handle("GET /api/integrations/google-calendar/connect", accessMW(http.HandlerFunc(h.googleCalendarConnect)))
	mux.HandleFunc("GET /api/auth/google", h.googleLoginStart)
	mux.HandleFunc("POST /api/auth/google/complete", h.googleLoginComplete)
	mux.HandleFunc("POST /api/auth/google/session", h.googleLoginWithIDToken)
	mux.HandleFunc("GET /api/integrations/google-calendar/callback", h.googleCalendarCallback)
	mux.Handle("DELETE /api/integrations/google-calendar", accessMW(http.HandlerFunc(h.googleCalendarDisconnect)))

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
