package wechat

import (
	"errors"
	"net/http"
	"time"
)

// ErrArticleUnavailable reports an article page without a usable body,
// which usually means the article was deleted or blocked by anti-crawler
// verification.
var ErrArticleUnavailable = errors.New("wechat: article content unavailable")

// DefaultTimeout is applied when a Client is created without one.
const DefaultTimeout = 15 * time.Second

// Client fetches public article pages. BaseURL and HTTPClient are exported
// for httptest-based callers; NewClient presets them.
type Client struct {
	BaseURL    string
	HTTPClient *http.Client
}

// NewClient builds a client for public article pages. No login state is
// involved: mp.weixin.qq.com article pages are readable anonymously.
func NewClient() *Client {
	return &Client{
		BaseURL:    MPBaseURL,
		HTTPClient: &http.Client{Timeout: DefaultTimeout},
	}
}

// Account is the public shape of one resolved official account. FakeID is
// the `biz` fakeid extracted from an article page.
type Account struct {
	FakeID string `json:"fake_id"`
	Name   string `json:"name"`
	Avatar string `json:"avatar"`
	Alias  string `json:"alias"`
}

// Article is a parsed public article page. HTML and MediaURLs are populated
// by FetchArticle.
type Article struct {
	Title         string    `json:"title"`
	Link          string    `json:"link"`
	Author        string    `json:"author"`
	PublishedAt   time.Time `json:"published_at"`
	HasPublished  bool      `json:"has_published"` // PublishedAt is populated.
	HTML          string    `json:"html"`
	MediaURLs     []string  `json:"media_urls"`
	AccountName   string    `json:"account_name,omitempty"`
	AccountAvatar string    `json:"account_avatar,omitempty"`
	Biz           string    `json:"biz,omitempty"`            // Official-account fakeid extracted from the article page.
	PublishedText string    `json:"published_text,omitempty"` // Raw #publish_time text when parsing fails.
}
