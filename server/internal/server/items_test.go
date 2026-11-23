package server

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"rill/internal/fetch"
)

func TestEmailTargetSourceKeepsFeedToken(t *testing.T) {
	app, _ := newTestApp(t, false)
	ctx := context.Background()
	if _, err := app.createSourceRow(ctx, sourceCreateInput{
		Kind: "rss", Name: "Mailbox", FeedURL: "imap://user@example.com",
	}); err != nil {
		t.Fatalf("createSourceRow: %v", err)
	}
	target, err := app.emailTargetSource(ctx, internalEmailAccount{Username: "user@example.com"})
	if err != nil {
		t.Fatalf("emailTargetSource: %v", err)
	}
	if target.FeedToken == "" {
		t.Fatal("existing mailbox source lost feed_token; media URLs would be /media//{hash}")
	}
	item := fetch.BuildEmailFeedItem(fetch.EmailMessage{
		MessageID: "<token@example.com>",
		FromAddr:  "news@example.com",
		Subject:   "token",
		HTML:      `<p><img src="https://cdn.example.com/logo.png"></p>`,
	}, target.FeedToken)
	if !strings.Contains(item.ContentHTML, "/media/"+target.FeedToken+"/") {
		t.Fatalf("email content must use the source feed token: %s", item.ContentHTML)
	}
}

func TestGetItemRepairsLegacyEmailMediaToken(t *testing.T) {
	app, client := newTestApp(t, true)
	client.login()
	ctx := context.Background()
	source, err := app.createSourceRow(ctx, sourceCreateInput{
		Kind: "rss", Name: "Mailbox", FeedURL: "imap://user@example.com",
	})
	if err != nil {
		t.Fatalf("createSourceRow: %v", err)
	}
	token, err := app.sourceFeedToken(ctx, source.ID)
	if err != nil {
		t.Fatal(err)
	}
	first := "https://cdn.example.com/one.png"
	second := "https://cdn.example.com/two.png"
	content := `<p><img src="/media//` + fetch.URLHash(first) + `"><img src="/media//` + fetch.URLHash(second) + `" data-remote-src="` + second + `"></p>`
	media := `{"images":["` + first + `","` + second + `"],"email_sender":"news@example.com"}`
	now := time.Now().UTC().Format(timeRFC3339)
	result, err := app.db.Exec(`
		INSERT INTO items (source_id, guid, title, content_html, content_text, media_json, fetched_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		source.ID, "legacy-media", "Legacy", content, "Legacy", media, now, now)
	if err != nil {
		t.Fatal(err)
	}
	itemID, _ := result.LastInsertId()

	var detail ItemDetail
	client.decode(http.MethodGet, "/api/v1/items/"+itoa(itemID), nil, false, http.StatusOK, &detail)
	if strings.Contains(detail.ContentHTML, "/media//") {
		t.Fatalf("legacy empty media token was not repaired: %s", detail.ContentHTML)
	}
	if !strings.Contains(detail.ContentHTML, "/media/"+token+"/") {
		t.Fatalf("media token not rewritten to current source token: %s", detail.ContentHTML)
	}
	if strings.Count(detail.ContentHTML, `data-remote-src=`) != 2 {
		t.Fatalf("data-remote-src should be backfilled for both images: %s", detail.ContentHTML)
	}
	if detail.CoverURL == nil || !strings.HasPrefix(*detail.CoverURL, "/media/"+token+"/") {
		t.Fatalf("cover URL should use the repaired media token: %v", detail.CoverURL)
	}
}

func TestPatchItemEmailImagesPersists(t *testing.T) {
	app, client := newTestApp(t, true)
	client.login()

	var source Source
	client.decode(http.MethodPost, "/api/v1/sources", map[string]any{
		"kind": "rss", "name": "Mailbox", "feed_url": "imap://user@example.com",
	}, true, http.StatusCreated, &source)
	now := time.Now().UTC().Format(timeRFC3339)
	result, err := app.db.Exec(`
		INSERT INTO items (source_id, guid, title, content_html, content_text, media_json, fetched_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		source.ID, "mail-1", "Weekly", `<p><img src="/media/token/hash" data-remote-src="https://cdn.example.com/a.png"></p>`,
		"Weekly", `{"images":["/media/token/hash"],"email_sender":"news@example.com"}`, now, now)
	if err != nil {
		t.Fatal(err)
	}
	itemID, _ := result.LastInsertId()

	var detail ItemDetail
	client.decode(http.MethodGet, "/api/v1/items/"+itoa(itemID), nil, false, http.StatusOK, &detail)
	if detail.EmailImagesAllowed || detail.SenderEmail != "news@example.com" {
		t.Fatalf("initial email detail: %+v", detail)
	}

	var response map[string]bool
	client.decode(http.MethodPatch, "/api/v1/items/"+itoa(itemID)+"/email-images", map[string]any{
		"allowed": true,
	}, true, http.StatusOK, &response)
	if !response["email_images_allowed"] {
		t.Fatalf("patch response: %+v", response)
	}

	client.decode(http.MethodGet, "/api/v1/items/"+itoa(itemID), nil, false, http.StatusOK, &detail)
	if !detail.EmailImagesAllowed {
		t.Fatalf("email image allow was not persisted: %+v", detail)
	}
}

func TestPatchItemEmailImagesRejectsNonEmail(t *testing.T) {
	app, client := newTestApp(t, true)
	client.login()
	source := createTestSource(t, client, "Web", "https://example.com/feed.xml")
	now := time.Now().UTC().Format(timeRFC3339)
	result, err := app.db.Exec(`
		INSERT INTO items (source_id, guid, title, content_text, fetched_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)`, source.ID, "web-1", "Post", "body", now, now)
	if err != nil {
		t.Fatal(err)
	}
	itemID, _ := result.LastInsertId()
	response, data := client.request(http.MethodPatch, "/api/v1/items/"+itoa(itemID)+"/email-images", map[string]any{
		"allowed": true,
	}, true)
	if response.StatusCode != http.StatusBadRequest || errorCode(t, data) != "not_email_item" {
		t.Fatalf("non-email item: %d %s", response.StatusCode, data)
	}
}

func TestFirstProxiedImage(t *testing.T) {
	cases := []struct {
		name string
		html string
		want string
	}{
		{
			name: "first proxied image",
			html: `<p>Intro</p><img src="/media/token/abc123" alt=""><img src="/media/token/def456">`,
			want: "/media/token/abc123",
		},
		{
			name: "no image",
			html: `<p>Only text</p>`,
			want: "",
		},
		{
			name: "external image is ignored",
			html: `<img src="https://example.com/cover.jpg">`,
			want: "",
		},
		{
			name: "unterminated attribute",
			html: `<img src="/media/token/abc123>`,
			want: "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := firstProxiedImage(tc.html)
			if tc.want == "" {
				if got != nil {
					t.Fatalf("expected nil, got %q", *got)
				}
				return
			}
			if got == nil || *got != tc.want {
				t.Fatalf("expected %q, got %v", tc.want, got)
			}
		})
	}
}
