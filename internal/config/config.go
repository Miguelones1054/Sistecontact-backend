package config

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Port                    string
	GoogleMapsAPIKey        string
	GoogleMapsLanguage      string
	GoogleMapsRegion        string
	GoogleHTTPTimeout       time.Duration
	CacheTTL                time.Duration
	CacheCleanup            time.Duration
	MaxPages                int
	GridMaxDepth            int
	SearchWorkers           int
	FirebaseCredentialsFile string
	GoogleOAuthClientID     string
	GoogleOAuthClientSecret string
	GoogleOAuthRedirectURL  string
	GoogleOAuthStateSecret  string
	FrontendOrigin          string
	CalendarTimeZone        string
}

func Load() (Config, error) {
	// Carga .env si existe (no sobrescribe variables ya presentes en el entorno).
	if err := loadDotEnv(".env"); err != nil {
		// No es fatal: en producción las vars vienen del entorno.
		_ = err
	}

	apiKey := os.Getenv("GOOGLE_MAPS_API_KEY")
	if apiKey == "" {
		return Config{}, fmt.Errorf("GOOGLE_MAPS_API_KEY es obligatorio")
	}

	cfg := Config{
		Port:                    env("PORT", "8080"),
		GoogleMapsAPIKey:        apiKey,
		GoogleMapsLanguage:      env("GOOGLE_MAPS_LANGUAGE", "es"),
		GoogleMapsRegion:        env("GOOGLE_MAPS_REGION", "CO"),
		GoogleHTTPTimeout:       seconds("GOOGLE_HTTP_TIMEOUT", 10),
		CacheTTL:                seconds("CACHE_TTL_SECONDS", 3600),
		CacheCleanup:            seconds("CACHE_CLEANUP_SECONDS", 600),
		MaxPages:                intEnv("MAX_PAGES", 3),
		GridMaxDepth:            intEnv("GRID_MAX_DEPTH", 3),
		SearchWorkers:           intEnv("SEARCH_WORKERS", 4),
		FirebaseCredentialsFile: env("FIREBASE_CREDENTIALS_FILE", "sistecontact-firebase-adminsdk-fbsvc-8adb9b6483.json"),
		GoogleOAuthClientID:     os.Getenv("GOOGLE_OAUTH_CLIENT_ID"),
		GoogleOAuthClientSecret: os.Getenv("GOOGLE_OAUTH_CLIENT_SECRET"),
		GoogleOAuthRedirectURL: env(
			"GOOGLE_OAUTH_REDIRECT_URL",
			"https://apisistecontact.nodefex.com/api/integrations/google-calendar/callback",
		),
		GoogleOAuthStateSecret: os.Getenv("GOOGLE_OAUTH_STATE_SECRET"),
		FrontendOrigin:         env("FRONTEND_ORIGIN", "https://sistecontact.nodefex.com"),
		CalendarTimeZone:       env("CALENDAR_TIMEZONE", "America/Bogota"),
	}
	return cfg, nil
}

// loadDotEnv parsea un archivo .env simple (CLAVE=valor) y define las variables
// en el entorno solo si no existen ya. Soporta líneas comentadas con #.
func loadDotEnv(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		value = strings.Trim(value, `"'`)
		if key == "" {
			continue
		}
		if _, exists := os.LookupEnv(key); !exists {
			_ = os.Setenv(key, value)
		}
	}
	return sc.Err()
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func seconds(key string, fallback int) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return time.Duration(fallback) * time.Second
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return time.Duration(fallback) * time.Second
	}
	return time.Duration(n) * time.Second
}

func intEnv(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}
