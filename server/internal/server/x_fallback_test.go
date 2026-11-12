package server

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

const xFallbackTestFeed = `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0"><channel><title>Fallback Feed</title><link>https://x.com/example_user</link>
<item><title>fallback item</title><link>https://x.com/example_user/status/1</link><guid>https://x.com/example_user/status/1</guid><pubDate>Fri, 31 Oct 2026 05:13:48 +0000</pubDate><description>hello</description></item>
</channel></rss>`

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func createXTestSource(t *testing.T, client *testClient) Source {
	t.Helper()
	var source Source
	client.decode(http.MethodPost, "/api/v1/sources", map[string]any{
		"kind":   "x",
		"name":   "@example_user",
		"handle": "example_user",
	}, true, http.StatusCreated, &source)
	return source
}

func xFeedResponse(req *http.Request) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/rss+xml"}},
		Body:       io.NopCloser(strings.NewReader(xFallbackTestFeed)),
		Request:    req,
	}
}

func TestXAutoFallbackToFixupx(t *testing.T) {
	app, client := newTestApp(t, false)
	client.login()
	source := createXTestSource(t, client)

	var primaryCount int
	app.fetcher.HTTP = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if strings.EqualFold(req.URL.Hostname(), "fixupx.com") {
			return xFeedResponse(req), nil
		}
		primaryCount++
		return nil, errors.New("primary host down")
	})}

	var fetched Source
	client.decode(http.MethodPost, "/api/v1/sources/"+itoa(source.ID)+"/fetch", nil, true, http.StatusOK, &fetched)
	if primaryCount != 1 {
		t.Fatalf("manual refresh primary attempts = %d, want 1", primaryCount)
	}
	if fetched.ItemCount != 1 || fetched.Note == nil || !strings.Contains(*fetched.Note, "fixupx.com") {
		t.Fatalf("fallback fetch = %+v note=%v", fetched, fetched.Note)
	}
}

func TestXAutoRefreshRetriesTwiceBeforeFixupx(t *testing.T) {
	app, client := newTestApp(t, false)
	client.login()
	source := createXTestSource(t, client)
	app.xFetchRetryDelay = time.Millisecond

	var primaryCount, fallbackCount int
	app.fetcher.HTTP = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if strings.EqualFold(req.URL.Hostname(), "fixupx.com") {
			fallbackCount++
			return xFeedResponse(req), nil
		}
		primaryCount++
		return nil, errors.New("primary host down")
	})}

	fetched, err := app.fetchSource(context.Background(), source.ID, false, "")
	if err != nil {
		t.Fatal(err)
	}
	if primaryCount != 3 {
		t.Fatalf("automatic primary attempts = %d, want 3 (initial + 2 retries)", primaryCount)
	}
	if fallbackCount != 1 {
		t.Fatalf("fallback attempts = %d, want 1", fallbackCount)
	}
	if fetched.Note == nil || !strings.Contains(*fetched.Note, "fixupx.com") {
		t.Fatalf("note = %v, want fixupx fallback", fetched.Note)
	}
}

func TestXAutoRefreshMarksAttentionAfterRetries(t *testing.T) {
	app, client := newTestApp(t, false)
	client.login()
	source := createXTestSource(t, client)
	app.xFetchRetryDelay = time.Millisecond

	var primaryCount, fallbackCount int
	app.fetcher.HTTP = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if strings.EqualFold(req.URL.Hostname(), "fixupx.com") {
			fallbackCount++
		} else {
			primaryCount++
		}
		return nil, errors.New("host down")
	})}

	if _, err := app.fetchSource(context.Background(), source.ID, false, ""); err == nil {
		t.Fatal("expected automatic fetch to fail")
	}
	if primaryCount != 3 || fallbackCount != 1 {
		t.Fatalf("attempts primary=%d fallback=%d, want 3/1", primaryCount, fallbackCount)
	}
	stored, err := app.getSourceRow(context.Background(), source.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.LastStatus == nil || *stored.LastStatus != "error" || stored.FailCount != 1 {
		t.Fatalf("source not marked for attention: status=%v fail=%d", stored.LastStatus, stored.FailCount)
	}
}
