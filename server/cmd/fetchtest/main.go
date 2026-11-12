// fetchtest is a small diagnostic CLI: it fetches a URL with the same
// fetch.Client rules (timeouts, redirects, size caps) as the running app and
// prints the raw response facts. Usage: go run ./cmd/fetchtest <url>
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"rill/internal/fetch"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: fetchtest <url>")
		os.Exit(2)
	}
	c := fetch.NewClient("Rill-fetchtest/0.1")
	ctx := context.Background()
	url := os.Args[1]
	fmt.Println("validate:", c.ValidateURL(ctx, url))
	res, err := c.GetBytes(ctx, url, 5<<20, http.Header{})
	if err != nil {
		fmt.Println("getbytes err:", err)
		os.Exit(1)
	}
	fmt.Println("status", res.StatusCode, "ct", res.ContentType, "final", res.FinalURL, "len", len(res.Body))
	fmt.Println("etag", res.ETag, "last-modified", res.LastModified)
}
