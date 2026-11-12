package server

import "testing"

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
