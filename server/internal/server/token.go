package server

import "rill/internal/secure"

func newFeedToken() (string, error) {
	return secure.RandomToken()
}

func maskToken(token string) string {
	return secure.MaskToken(token)
}
