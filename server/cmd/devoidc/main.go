// Command devoidc is a development-only fake OIDC provider for running Rill
// locally without a Lazycat microserver. It implements just enough of the
// authorization-code + PKCE flow for Rill's login:
//
//	GET  /authorize  → 302 back to the client with ?code=&state=
//	POST /token      → JSON {access_token, token_type, id_token}
//	GET  /userinfo   → JSON {sub, preferred_username, name, email}
//
// It performs no real authentication and must never be exposed beyond
// loopback. Start it together with the server (see server/README.md):
//
//	go run ./cmd/devoidc            # listens on 127.0.0.1:8091
//	RILL_OIDC_CLIENT_ID=dev \
//	RILL_OIDC_CLIENT_SECRET=dev \
//	RILL_OIDC_AUTH_URI=http://127.0.0.1:8091/authorize \
//	RILL_OIDC_TOKEN_URI=http://127.0.0.1:8091/token \
//	RILL_OIDC_USERINFO_URI=http://127.0.0.1:8091/userinfo \
//	go run ./cmd/rill
package main

import (
	"fmt"
	"log"
	"net/http"
	"strings"
)

const listen = "127.0.0.1:8091"

func main() {
	mux := http.NewServeMux()

	// Auto-approve every authorization request with a fixed dev code.
	mux.HandleFunc("GET /authorize", func(w http.ResponseWriter, r *http.Request) {
		redirect := r.URL.Query().Get("redirect_uri")
		state := r.URL.Query().Get("state")
		if redirect == "" {
			http.Error(w, "missing redirect_uri", http.StatusBadRequest)
			return
		}
		separator := "?"
		if strings.Contains(redirect, "?") {
			separator = "&"
		}
		http.Redirect(w, r, fmt.Sprintf("%s%scode=dev-code&state=%s", redirect, separator, state), http.StatusFound)
	})

	mux.HandleFunc("POST /token", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"dev-access-token","token_type":"Bearer","id_token":"dev-id-token"}`))
	})

	mux.HandleFunc("GET /userinfo", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer dev-access-token" {
			http.Error(w, "invalid token", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"sub":"dev-user","preferred_username":"dev","name":"本地开发","email":"dev@localhost"}`))
	})

	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	})

	log.Printf("devoidc (fake OIDC provider) listening on http://%s", listen)
	log.Printf("authorize : http://%s/authorize", listen)
	log.Printf("token     : http://%s/token", listen)
	log.Printf("userinfo  : http://%s/userinfo", listen)
	if err := http.ListenAndServe(listen, mux); err != nil {
		log.Fatal(err)
	}
}
