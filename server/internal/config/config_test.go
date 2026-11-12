package config

import (
	"testing"
)

func TestLoadRequiresOIDC(t *testing.T) {
	t.Setenv("RILL_DATA_DIR", t.TempDir())
	t.Setenv("RILL_CACHE_DIR", t.TempDir())
	t.Setenv("RILL_OIDC_CLIENT_ID", "")
	t.Setenv("RILL_OIDC_CLIENT_SECRET", "")
	t.Setenv("RILL_OIDC_AUTH_URI", "")
	t.Setenv("RILL_OIDC_TOKEN_URI", "")
	t.Setenv("RILL_OIDC_USERINFO_URI", "")
	if _, err := Load(); err == nil {
		t.Fatal("incomplete OIDC configuration accepted")
	}

	t.Setenv("RILL_OIDC_CLIENT_ID", "client-id")
	t.Setenv("RILL_OIDC_CLIENT_SECRET", "client-secret")
	t.Setenv("RILL_OIDC_AUTH_URI", "https://box.example/sys/oauth/auth")
	t.Setenv("RILL_OIDC_TOKEN_URI", "https://box.example/sys/oauth/token")
	t.Setenv("RILL_OIDC_USERINFO_URI", "https://box.example/sys/oauth/userinfo")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.OIDC.Scopes != "openid profile email" {
		t.Fatalf("unexpected default scopes: %q", cfg.OIDC.Scopes)
	}
}

func TestLoadSecureCookiePolicy(t *testing.T) {
	t.Setenv("RILL_OIDC_CLIENT_ID", "client-id")
	t.Setenv("RILL_OIDC_CLIENT_SECRET", "client-secret")
	t.Setenv("RILL_OIDC_AUTH_URI", "https://box.example/sys/oauth/auth")
	t.Setenv("RILL_OIDC_TOKEN_URI", "https://box.example/sys/oauth/token")
	t.Setenv("RILL_OIDC_USERINFO_URI", "https://box.example/sys/oauth/userinfo")
	t.Setenv("RILL_DATA_DIR", t.TempDir())
	t.Setenv("RILL_CACHE_DIR", t.TempDir())

	t.Setenv("RILL_LISTEN", "127.0.0.1:8080")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.CookieSecure {
		t.Fatal("loopback development should allow a non-secure cookie")
	}

	t.Setenv("RILL_LISTEN", "0.0.0.0:8080")
	cfg, err = Load()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.CookieSecure {
		t.Fatal("non-loopback listen should default to Secure cookies")
	}

	t.Setenv("RILL_COOKIE_SECURE", "false")
	cfg, err = Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.CookieSecure {
		t.Fatal("explicit RILL_COOKIE_SECURE=false was ignored")
	}
}
