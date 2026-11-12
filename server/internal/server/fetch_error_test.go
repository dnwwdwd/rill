package server

import (
	"strings"
	"testing"

	"rill/internal/fetch"
)

func TestReadableFetchErrorHTTPStatuses(t *testing.T) {
	cases := []struct {
		status int
		want   string
	}{
		{status: 429, want: "限流"},
		{status: 403, want: "拒绝访问"},
		{status: 404, want: "404"},
	}
	for _, tc := range cases {
		got := readableFetchError(&fetch.HTTPStatusError{Status: tc.status}, "x")
		if !strings.Contains(got, tc.want) {
			t.Errorf("status %d: got %q, want it to contain %q", tc.status, got, tc.want)
		}
	}
}

func TestSourceHostLabel(t *testing.T) {
	if got := sourceHostLabel("https://fxtwitter.com/foo/feed.xml?count=20"); got != "fxtwitter.com" {
		t.Errorf("host = %q", got)
	}
	if got := sourceHostLabel("not a url"); got != "地址" {
		t.Errorf("fallback host = %q", got)
	}
}

func TestAutoXFallbackURL(t *testing.T) {
	got := autoXFallbackURL("https://fxtwitter.com/foo/feed.xml?count=20")
	want := "https://fixupx.com/foo/feed.xml?count=20"
	if got != want {
		t.Fatalf("fallback = %q, want %q", got, want)
	}
	if got := autoXFallbackURL("https://example.com/foo/feed.xml"); got != "" {
		t.Fatalf("non-X URL fallback = %q, want empty", got)
	}
}
