// imaptest is a read-only diagnostic CLI for the mailbox auto-sync channel.
// Credentials come from environment variables and are never printed or
// persisted. Usage:
//
//	RILL_IMAP_HOST=imap.qq.com RILL_IMAP_PORT=993 \
//	RILL_IMAP_USER=... RILL_IMAP_PASS=... go run ./cmd/imaptest [-fetch]
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strconv"
	"time"

	"rill/internal/fetch"
)

func main() {
	fetchMail := flag.Bool("fetch", false, "fetch the newest messages from syncable folders (read-only, BODY.PEEK) and print parsed summaries")
	limit := flag.Int("limit", 5, "max messages to fetch")
	flag.Parse()

	port, _ := strconv.Atoi(envOr("RILL_IMAP_PORT", "993"))
	cfg := fetch.IMAPConfig{
		Host:     os.Getenv("RILL_IMAP_HOST"),
		Port:     port,
		Username: os.Getenv("RILL_IMAP_USER"),
		Password: os.Getenv("RILL_IMAP_PASS"),
	}
	if cfg.Host == "" || cfg.Username == "" || cfg.Password == "" {
		fmt.Fprintln(os.Stderr, "missing RILL_IMAP_HOST / RILL_IMAP_USER / RILL_IMAP_PASS")
		os.Exit(2)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	start := time.Now()
	mailboxes, err := fetch.ListMailboxes(ctx, cfg)
	if err != nil {
		fmt.Println("LIST FAILED:", err)
		os.Exit(1)
	}
	fmt.Printf("LOGIN+LIST ok in %s; %d mailboxes\n", time.Since(start).Round(time.Millisecond), len(mailboxes))
	for i, name := range mailboxes {
		if i >= 8 {
			fmt.Printf("  ... and %d more\n", len(mailboxes)-8)
			break
		}
		fmt.Println("  -", name)
	}
	if !*fetchMail {
		return
	}

	start = time.Now()
	result, err := fetch.FetchNewEmails(ctx, cfg, fetch.EmailSyncState{}, *limit)
	if err != nil {
		fmt.Println("FETCH FAILED:", err)
		os.Exit(1)
	}
	for _, warning := range result.Warnings {
		fmt.Println("WARN:", warning)
	}
	fmt.Printf("SYNC fetch ok in %s; %d messages (read-only)\n", time.Since(start).Round(time.Millisecond), len(result.Emails))
	for _, email := range result.Emails {
		message, parseErr := fetch.ParseEmailMessage(email.Raw)
		if parseErr != nil {
			fmt.Printf("  folder=%q uid=%d parse error: %v\n", email.Mailbox, email.UID, parseErr)
			continue
		}
		item := fetch.BuildEmailFeedItem(message, "testtoken")
		fmt.Printf("  folder=%q uid=%d from=%q subject=%q date=%s html=%dB inline=%d title=%q media=%d\n",
			email.Mailbox, email.UID, message.FromAddr, message.Subject, message.Date.Format(time.RFC3339),
			len(message.HTML), len(message.Inline), item.Title, len(item.MediaURLs))
	}
}

func envOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
