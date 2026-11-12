package server

import "testing"

func TestNormalizeRSSHubURL(t *testing.T) {
	cases := []struct{ in, want string }{
		{"rsshub://baoyu/blog", "https://rsshub.app/baoyu/blog"},
		{"rsshub://baoyu/blog?limit=10", "https://rsshub.app/baoyu/blog?limit=10"},
		{"RSSHub://Baoyu/blog/", "https://rsshub.app/Baoyu/blog"},
		{"rsshub://rsshub.example.com/baoyu/blog", "https://rsshub.example.com/baoyu/blog"},
		{"rsshub://rsshub.rssforever.com/60s/world", "https://rsshub.rssforever.com/60s/world"},
		{"https://example.com/feed.xml", "https://example.com/feed.xml"},
		{"https://innei.in/feed", "https://innei.in/feed"},
		{"rsshub://", "rsshub://"},
		{"", ""},
	}
	for _, tc := range cases {
		if got := normalizeRSSHubURL(tc.in); got != tc.want {
			t.Fatalf("normalizeRSSHubURL(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
