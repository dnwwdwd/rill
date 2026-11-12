package config

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// OIDCConfig contains the Lazycat-provided OIDC client and endpoint values.
type OIDCConfig struct {
	Issuer       string
	ClientID     string
	ClientSecret string
	AuthURI      string
	TokenURI     string
	UserinfoURI  string
	RedirectURL  string
	Scopes       string
}

// Config is the process-level configuration. It intentionally contains no
// persisted defaults beyond paths; mutable reader/AI settings live in SQLite.
type Config struct {
	Listen            string
	DataDir           string
	CacheDir          string
	OIDC              OIDCConfig
	CookieSecure      bool
	SessionTTL        time.Duration
	LogLevel          string
	MediaMaxBytes     int64
	WebDir            string
	OutboundUserAgent string
	// Timezone is the IANA zone used to interpret digest `run_at_local`
	// values. The container often runs in UTC, while the user configures the
	// report time in local wall-clock time.
	Timezone string
	// BestBlogsCatalogURL is the public OPML directory used to resolve
	// WeChat public-account names before falling back to MP login search.
	BestBlogsCatalogURL string
}

// Load reads RILL_* environment variables. Authentication is OIDC-only since
// DEC-017: the Lazycat-provided OIDC client and endpoint variables are
// mandatory. Failing at startup is safer than silently creating an
// unauthenticated app. Local development can point these variables at
// `go run ./cmd/devoidc`, the bundled fake provider.
func Load() (Config, error) {
	cfg := Config{
		Listen:   env("RILL_LISTEN", "127.0.0.1:8080"),
		DataDir:  env("RILL_DATA_DIR", "/lzcapp/var/data"),
		CacheDir: env("RILL_CACHE_DIR", "/lzcapp/cache/media"),
		OIDC: OIDCConfig{
			Issuer:       env("RILL_OIDC_ISSUER", ""),
			ClientID:     env("RILL_OIDC_CLIENT_ID", ""),
			ClientSecret: env("RILL_OIDC_CLIENT_SECRET", ""),
			AuthURI:      env("RILL_OIDC_AUTH_URI", ""),
			TokenURI:     env("RILL_OIDC_TOKEN_URI", ""),
			UserinfoURI:  env("RILL_OIDC_USERINFO_URI", ""),
			RedirectURL:  env("RILL_OIDC_REDIRECT_URL", ""),
			Scopes:       strings.TrimSpace(env("RILL_OIDC_SCOPES", "openid profile email")),
		},
		SessionTTL:        7 * 24 * time.Hour,
		LogLevel:          env("RILL_LOG_LEVEL", "info"),
		MediaMaxBytes:     envInt64("RILL_MEDIA_MAX_BYTES", 2<<30),
		WebDir:            os.Getenv("RILL_WEB_DIR"),
		OutboundUserAgent: env("RILL_USER_AGENT", "Rill/0.1"),
		// Microserver containers usually run in UTC. Defaulting to the
		// deployment's local zone keeps `run_at_local` wall-clock accurate.
		Timezone: env("RILL_TIMEZONE", "Asia/Shanghai"),
		BestBlogsCatalogURL: env("RILL_BESTBLOGS_CATALOG_URL",
			"https://cdn.jsdelivr.net/gh/ginobefun/BestBlogs@main/opml/bestblogs_wechat2rss_opml_all.opml"),
	}
	missing := make([]string, 0, 5)
	for _, field := range []struct {
		name  string
		value string
	}{
		{"RILL_OIDC_CLIENT_ID", cfg.OIDC.ClientID},
		{"RILL_OIDC_CLIENT_SECRET", cfg.OIDC.ClientSecret},
		{"RILL_OIDC_AUTH_URI", cfg.OIDC.AuthURI},
		{"RILL_OIDC_TOKEN_URI", cfg.OIDC.TokenURI},
		{"RILL_OIDC_USERINFO_URI", cfg.OIDC.UserinfoURI},
	} {
		if strings.TrimSpace(field.value) == "" {
			missing = append(missing, field.name)
		}
	}
	if len(missing) > 0 {
		return Config{}, fmt.Errorf("OIDC login requires %s (local development: `go run ./cmd/devoidc`)", strings.Join(missing, ", "))
	}
	if cfg.OIDC.Scopes == "" {
		cfg.OIDC.Scopes = "openid profile email"
	}
	if cfg.Listen == "" {
		cfg.Listen = "127.0.0.1:8080"
	}
	// Secure cookies are required in production. Loopback development stays
	// usable over plain HTTP; an operator can always override explicitly.
	if _, explicit := os.LookupEnv("RILL_COOKIE_SECURE"); explicit {
		cfg.CookieSecure = envBool("RILL_COOKIE_SECURE", false)
	} else {
		cfg.CookieSecure = !listenIsLoopback(cfg.Listen)
	}
	var err error
	if cfg.DataDir, err = filepath.Abs(cfg.DataDir); err != nil {
		return Config{}, fmt.Errorf("resolve data dir: %w", err)
	}
	if cfg.CacheDir, err = filepath.Abs(cfg.CacheDir); err != nil {
		return Config{}, fmt.Errorf("resolve cache dir: %w", err)
	}
	if cfg.WebDir != "" {
		cfg.WebDir, err = filepath.Abs(cfg.WebDir)
		if err != nil {
			return Config{}, fmt.Errorf("resolve web dir: %w", err)
		}
	}
	return cfg, nil
}

func env(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func listenIsLoopback(listen string) bool {
	host, _, err := net.SplitHostPort(listen)
	if err != nil {
		host = listen
	}
	host = strings.Trim(host, "[]")
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func envBool(key string, fallback bool) bool {
	raw, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(raw) == "" {
		return fallback
	}
	value, err := strconv.ParseBool(raw)
	if err != nil {
		return fallback
	}
	return value
}

func envInt64(key string, fallback int64) int64 {
	raw, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(raw) == "" {
		return fallback
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}
