package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

const sourceBatchMaxItems = 20

type sourceBatchCreateInput struct {
	Sources []sourceCreateInput `json:"sources"`
}

func (a *App) handleBatchCreateSources(w http.ResponseWriter, r *http.Request) error {
	var body sourceBatchCreateInput
	if err := decodeJSON(r, &body); err != nil {
		return err
	}
	if len(body.Sources) == 0 {
		return badRequest("sources_required", "请至少选择一个订阅源")
	}
	if len(body.Sources) > sourceBatchMaxItems {
		return badRequest("too_many_sources", "单次最多添加 20 个订阅源")
	}
	existing, err := a.existingSourceFeedURLs(r.Context())
	if err != nil {
		return internalError(err)
	}
	result := SourceBatchCreateResponse{
		Created: make([]Source, 0, len(body.Sources)),
		Errors:  make([]SourceBatchError, 0),
	}
	fetchIDs := make([]int64, 0, len(body.Sources))
	batchSeen := map[string]bool{}
	for index, input := range body.Sources {
		feedURL := sourceBatchFeedURL(input)
		if feedURL != "" {
			if sourceID, ok := existing[feedURL]; ok {
				result.Errors = append(result.Errors, SourceBatchError{
					Index:            index,
					Name:             strings.TrimSpace(input.Name),
					FeedURL:          feedURL,
					Code:             "source_exists",
					Message:          "该源已在订阅列表中",
					ExistingSourceID: sourceID,
				})
				continue
			}
			if batchSeen[feedURL] {
				result.Errors = append(result.Errors, SourceBatchError{
					Index:   index,
					Name:    strings.TrimSpace(input.Name),
					FeedURL: feedURL,
					Code:    "duplicate_source",
					Message: "本次提交中有重复的订阅地址",
				})
				continue
			}
			batchSeen[feedURL] = true
		}
		source, createErr := a.createSourceRow(r.Context(), input)
		if createErr != nil {
			result.Errors = append(result.Errors, sourceBatchError(index, input, createErr))
			continue
		}
		result.Created = append(result.Created, source)
		fetchIDs = append(fetchIDs, source.ID)
		if source.BackfillMode == "full" && !a.disableBackfill {
			a.startBackfill(source.ID)
		}
	}
	a.startInitialSourceFetches(fetchIDs)
	writeJSON(w, http.StatusOK, result)
	return nil
}

func (a *App) existingSourceFeedURLs(ctx context.Context) (map[string]int64, error) {
	rows, err := a.db.QueryContext(ctx, `SELECT id, feed_url FROM sources WHERE feed_url <> ''`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := map[string]int64{}
	for rows.Next() {
		var id int64
		var feedURL string
		if err := rows.Scan(&id, &feedURL); err != nil {
			return nil, err
		}
		key := strings.TrimSpace(feedURL)
		if key == "" {
			continue
		}
		if _, ok := result[key]; !ok {
			result[key] = id
		}
	}
	return result, rows.Err()
}

func sourceBatchFeedURL(input sourceCreateInput) string {
	feedURL := strings.TrimSpace(input.FeedURL)
	if feedURL == "" {
		feedURL = strings.TrimSpace(input.URL)
	}
	if strings.EqualFold(strings.TrimSpace(input.Kind), "x") {
		handle := strings.TrimPrefix(strings.TrimSpace(input.Handle), "@")
		if handle != "" {
			feedURL = fmt.Sprintf("https://fxtwitter.com/%s/feed.xml?count=20", handle)
		}
	}
	return normalizeRSSHubURL(feedURL)
}

func sourceBatchError(index int, input sourceCreateInput, err error) SourceBatchError {
	code := "internal_error"
	message := "创建订阅源失败"
	var apiErr *apiError
	if errors.As(err, &apiErr) {
		if apiErr.Code != "" {
			code = apiErr.Code
		}
		if apiErr.Message != "" {
			message = apiErr.Message
		}
	}
	return SourceBatchError{
		Index:   index,
		Name:    strings.TrimSpace(input.Name),
		FeedURL: sourceBatchFeedURL(input),
		Code:    code,
		Message: message,
	}
}

func (a *App) startInitialSourceFetches(sourceIDs []int64) {
	if a.disableInitialFetch || a.ctx == nil || len(sourceIDs) == 0 {
		return
	}
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		// Keep the batch path from opening 20 outbound fetches at once. The
		// scheduler uses the same ceiling for due sources.
		semaphore := make(chan struct{}, 4)
		var wg sync.WaitGroup
		for _, sourceID := range sourceIDs {
			wg.Add(1)
			go func(sourceID int64) {
				defer wg.Done()
				semaphore <- struct{}{}
				defer func() { <-semaphore }()
				ctx, cancel := context.WithTimeout(a.ctx, 20*time.Second)
				defer cancel()
				if _, err := a.fetchSource(ctx, sourceID, false, ""); err != nil {
					a.logger.Warn("initial batch source fetch failed", "source_id", sourceID, "err", err)
				}
			}(sourceID)
		}
		wg.Wait()
	}()
}
