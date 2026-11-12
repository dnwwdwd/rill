// bskytest 是一个通用抓取诊断 CLI（历史名称）：
//   go run ./cmd/bskytest <feed-url>        按 RSS 抓取解析
//   go run ./cmd/bskytest -x <handle>       按 FxTwitter 时间线翻页
//   go run ./cmd/bskytest -b <handle>       按 Bluesky AppView 翻页
package main

import (
	"context"
	"fmt"
	"os"

	"rill/internal/fetch"
)

func main() {
	args := os.Args[1:]
	c := fetch.NewClient("Rill-diag/0.1")
	ctx := context.Background()
	switch {
	case len(args) == 2 && args[0] == "-x":
		items, err := c.FetchXTimeline(ctx, args[1], "", nil)
		report(items, err)
	case len(args) == 2 && args[0] == "-b":
		items, err := c.FetchBskyTimeline(ctx, args[1], "", nil)
		report(items, err)
	case len(args) == 1:
		res, err := c.FetchFeed(ctx, args[0], fetch.FetchState{}, "diag-token")
		if err != nil {
			fmt.Println("err:", err)
			os.Exit(1)
		}
		fmt.Println("title:", res.Title, "items:", len(res.Items))
		for i, it := range res.Items {
			fmt.Printf("- %s | estimated=%v | %s | %s\n", it.PublishedAt.Format("2006-01-02 15:04"), it.PublishedEstimated, it.Title, it.URL)
			if i >= 4 {
				break
			}
		}
	default:
		fmt.Fprintln(os.Stderr, "usage: bskytest [-x handle | -b handle | feed-url]")
		os.Exit(2)
	}
}

func report(items []fetch.FeedItem, err error) {
	if err != nil {
		fmt.Println("err:", err)
		os.Exit(1)
	}
	fmt.Println("items:", len(items))
	for i, it := range items {
		fmt.Printf("- %s | %s | %s\n", it.PublishedAt.Format("2006-01-02 15:04"), it.Title, it.URL)
		if i >= 4 {
			break
		}
	}
}
