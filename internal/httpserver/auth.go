package httpserver

import (
	"context"
	"net/http"
	"strings"

	"firebase.google.com/go/v4/auth"
	"github.com/sistecontact/api/internal/model"
	"github.com/sistecontact/api/internal/usersettings"
)

type ctxKey string

const (
	uidKey         ctxKey = "uid"
	emailKey       ctxKey = "email"
	displayNameKey ctxKey = "display_name"
)

func uidFromContext(ctx context.Context) (string, bool) {
	uid, ok := ctx.Value(uidKey).(string)
	return uid, ok && uid != ""
}

func identityFromContext(ctx context.Context) (model.VisitorIdentity, bool) {
	uid, ok := uidFromContext(ctx)
	if !ok {
		return model.VisitorIdentity{}, false
	}
	email, _ := ctx.Value(emailKey).(string)
	name, _ := ctx.Value(displayNameKey).(string)
	return model.VisitorIdentity{
		UID:         uid,
		Email:       email,
		DisplayName: name,
	}, true
}

// requireAuth verifica el Bearer token de Firebase Auth.
func requireAuth(authClient *auth.Client) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			if !strings.HasPrefix(header, "Bearer ") {
				writeError(w, http.StatusUnauthorized, "token de autenticación requerido")
				return
			}
			token := strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
			if token == "" {
				writeError(w, http.StatusUnauthorized, "token de autenticación requerido")
				return
			}

			decoded, err := authClient.VerifyIDToken(r.Context(), token)
			if err != nil {
				writeError(w, http.StatusUnauthorized, "token inválido o expirado")
				return
			}

			email, _ := decoded.Claims["email"].(string)
			name, _ := decoded.Claims["name"].(string)

			ctx := context.WithValue(r.Context(), uidKey, decoded.UID)
			ctx = context.WithValue(ctx, emailKey, email)
			ctx = context.WithValue(ctx, displayNameKey, name)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// requireAccess exige users/{uid}/settings/access.sistecontact_enabled == true.
// Si el documento no existe, lo crea con false y deniega el acceso.
func requireAccess(settings *usersettings.Store) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			uid, ok := uidFromContext(r.Context())
			if !ok {
				writeError(w, http.StatusUnauthorized, "token de autenticación requerido")
				return
			}
			access, err := settings.GetOrCreateAccess(r.Context(), uid)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "no se pudo verificar el acceso")
				return
			}
			if !access.SistecontactEnabled {
				writeError(w, http.StatusForbidden, "no tienes membresía activa. Actívala en www.nodefex.com/sistecontact")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
