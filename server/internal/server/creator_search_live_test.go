//go:build live

package server

import (
	"context"
	"testing"
	"time"

	"rill/internal/fetch"
)

// Run with:
//
//	go test -tags live ./internal/server -run TestLiveCreatorSearch -v
//
// Public-network test; results are recorded in docs/RSS-SOURCES.md §2.15.
func TestLiveCreatorSearch(t *testing.T) {
	app := &App{
		fetcher:         fetch.NewClient("Rill/0.1 (live creator search)"),
		appleAPIBase:    "https://itunes.apple.com",
		bskyAppViewBase: "https://public.api.bsky.app",
		nowFn:           time.Now,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	t.Run("apple", func(t *testing.T) {
		candidates, err := app.searchAppleCreators(ctx, "这集我看过")
		if err != nil {
			t.Fatalf("searchAppleCreators: %v", err)
		}
		if len(candidates) == 0 {
			t.Fatal("no Apple candidates")
		}
		t.Logf("apple: %s | %s | match=%s", candidates[0].Name, candidates[0].FeedURL, candidates[0].Match)
	})

	t.Run("bluesky", func(t *testing.T) {
		candidates, err := app.searchBlueskyCreators(ctx, "jay.bsky.team")
		if err != nil {
			t.Fatalf("searchBlueskyCreators: %v", err)
		}
		if len(candidates) == 0 {
			t.Fatal("no Bluesky candidates")
		}
		t.Logf("bluesky: %s | %s | match=%s", candidates[0].Name, candidates[0].Handle, candidates[0].Match)
	})
}
