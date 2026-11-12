package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

const (
	creatorSearchTotalTimeout    = 8 * time.Second
	creatorSearchPlatformTimeout = 5 * time.Second
	creatorSearchCacheTTL        = 10 * time.Minute
	creatorSearchFailureCacheTTL = time.Minute
	creatorSearchCacheMaxEntries = 64
	creatorSearchMaxQueryRunes   = 80
	creatorSearchAppleLimit      = 5
	creatorSearchBlueskyLimit    = 5
	creatorSearchDefaultBskyBase = "https://public.api.bsky.app"
)

const (
	creatorSearchStatusOK      = "ok"
	creatorSearchStatusEmpty   = "empty"
	creatorSearchStatusError   = "error"
	creatorSearchStatusTimeout = "timeout"
)

var creatorSearchPlatformOrder = []string{"apple", "bluesky", "bestblogs"}

var creatorSearchPlatformPriorities = map[string]int{
	"apple":             0,
	"bluesky":           1,
	"bestblogs":         3,
	"bestblogs-podcast": 4,
	"bestblogs-x":       5,
	"bestblogs-youtube": 6,
}

type creatorSearchRequest struct {
	Query     string   `json:"query"`
	Platforms []string `json:"platforms"`
}

type creatorSearchCacheEntry struct {
	response CreatorSearchResponse
	expires  time.Time
}

type creatorSearchGroup struct {
	platform string
	label    string
	run      func(context.Context, string) ([]CreatorSearchCandidate, error)
}

type appleSearchResponse struct {
	ResultCount int `json:"resultCount"`
	Results     []struct {
		CollectionName    string `json:"collectionName"`
		TrackName         string `json:"trackName"`
		ArtistName        string `json:"artistName"`
		FeedURL           string `json:"feedUrl"`
		CollectionViewURL string `json:"collectionViewUrl"`
		ArtworkURL600     string `json:"artworkUrl600"`
		ArtworkURL100     string `json:"artworkUrl100"`
	} `json:"results"`
}

type bskyActorSearchResponse struct {
	Actors []struct {
		DID         string `json:"did"`
		Handle      string `json:"handle"`
		DisplayName string `json:"displayName"`
		Description string `json:"description"`
		Avatar      string `json:"avatar"`
	} `json:"actors"`
}

func (a *App) handleSearchCreators(w http.ResponseWriter, r *http.Request) error {
	var body creatorSearchRequest
	if err := decodeJSON(r, &body); err != nil {
		return err
	}
	query := normalizeCreatorSearchQuery(body.Query)
	if query == "" {
		return badRequest("query_required", "请输入创作者名称")
	}
	if utf8.RuneCountInString(query) > creatorSearchMaxQueryRunes {
		return badRequest("query_too_long", "创作者名称不能超过 80 字")
	}
	platforms, err := normalizeCreatorSearchPlatforms(body.Platforms)
	if err != nil {
		return err
	}
	response, err := a.searchCreators(r.Context(), query, platforms)
	if err != nil {
		return internalError(err)
	}
	writeJSON(w, http.StatusOK, response)
	return nil
}

func normalizeCreatorSearchQuery(raw string) string {
	value := strings.TrimSpace(raw)
	value = strings.TrimPrefix(value, "@")
	return strings.TrimSpace(value)
}

func normalizeCreatorSearchPlatforms(raw []string) ([]string, error) {
	if len(raw) == 0 {
		return append([]string(nil), creatorSearchPlatformOrder...), nil
	}
	seen := map[string]bool{}
	result := make([]string, 0, len(raw))
	for _, item := range raw {
		platform := strings.ToLower(strings.TrimSpace(item))
		if platform == "" {
			continue
		}
		if !knownCreatorSearchPlatform(platform) {
			return nil, badRequest("invalid_platform", "不支持的搜索平台："+platform)
		}
		if seen[platform] {
			continue
		}
		seen[platform] = true
		result = append(result, platform)
	}
	if len(result) == 0 {
		return nil, badRequest("invalid_platform", "请选择至少一个搜索平台")
	}
	return result, nil
}

func knownCreatorSearchPlatform(platform string) bool {
	for _, candidate := range creatorSearchPlatformOrder {
		if candidate == platform {
			return true
		}
	}
	return false
}

func (a *App) searchCreators(ctx context.Context, query string, platforms []string) (CreatorSearchResponse, error) {
	cacheKey := creatorSearchCacheKey(query, platforms)
	if cached, ok := a.loadCreatorSearchCache(cacheKey); ok {
		cached.Query = query
		a.markExistingCreatorSources(ctx, cached.Candidates)
		return cached, nil
	}

	selected := make(map[string]bool, len(platforms))
	for _, platform := range platforms {
		selected[platform] = true
	}
	groups := []creatorSearchGroup{
		{platform: "apple", label: "Apple Podcasts", run: a.searchAppleCreators},
		{platform: "bluesky", label: "Bluesky", run: a.searchBlueskyCreators},
		{platform: "bestblogs", label: "BestBlogs 目录", run: a.searchBestBlogsCreators},
	}

	searchCtx, cancel := context.WithTimeout(ctx, creatorSearchTotalTimeout)
	defer cancel()

	statuses := make([]CreatorSearchPlatformStatus, len(groups))
	candidateGroups := make([][]CreatorSearchCandidate, len(groups))
	var wg sync.WaitGroup
	for index := range groups {
		group := groups[index]
		if !selected[group.platform] {
			continue
		}
		wg.Add(1)
		go func(index int, group creatorSearchGroup) {
			defer wg.Done()
			platformCtx, platformCancel := context.WithTimeout(searchCtx, creatorSearchPlatformTimeout)
			defer platformCancel()
			candidates, err := group.run(platformCtx, query)
			status := CreatorSearchPlatformStatus{
				Platform: group.platform,
				Label:    group.label,
			}
			switch {
			case err != nil:
				status.Status = creatorSearchStatusForError(platformCtx, err)
				status.Error = creatorSearchErrorMessage(status.Status, err)
			case len(candidates) == 0:
				status.Status = creatorSearchStatusEmpty
			default:
				status.Status = creatorSearchStatusOK
				status.Count = len(candidates)
				candidateGroups[index] = candidates
			}
			statuses[index] = status
		}(index, group)
	}
	wg.Wait()

	response := CreatorSearchResponse{
		Query:      query,
		Candidates: mergeCreatorSearchCandidates(candidateGroups),
		Platforms:  make([]CreatorSearchPlatformStatus, 0, len(groups)),
	}
	for index := range groups {
		if !selected[groups[index].platform] {
			continue
		}
		response.Platforms = append(response.Platforms, statuses[index])
	}
	a.markExistingCreatorSources(ctx, response.Candidates)
	if err := a.prepareCreatorSearchIcons(ctx, response.Candidates); err != nil {
		a.logger.Warn("candidate icon registration failed", "err", err)
	}
	a.storeCreatorSearchCache(cacheKey, response)
	return response, nil
}

// prepareCreatorSearchIcons registers candidate avatars in the media cache and
// fills IconProxyURL for display; icon_url is untouched for creation.
func (a *App) prepareCreatorSearchIcons(ctx context.Context, candidates []CreatorSearchCandidate) error {
	iconURLs := make([]string, 0, len(candidates))
	for index := range candidates {
		if proxy := candidateIconProxyURL(candidates[index].IconURL); proxy != "" {
			candidates[index].IconProxyURL = proxy
			iconURLs = append(iconURLs, candidates[index].IconURL)
		}
	}
	if len(iconURLs) == 0 {
		return nil
	}
	return a.prepareCandidateIcons(ctx, iconURLs)
}

func creatorSearchStatusForError(ctx context.Context, err error) string {
	if errors.Is(ctx.Err(), context.DeadlineExceeded) || errors.Is(err, context.DeadlineExceeded) {
		return creatorSearchStatusTimeout
	}
	return creatorSearchStatusError
}

func creatorSearchErrorMessage(status string, err error) string {
	if status == creatorSearchStatusTimeout {
		return "搜索超时"
	}
	message := strings.TrimSpace(err.Error())
	if message == "" {
		return "搜索失败"
	}
	return message
}

func mergeCreatorSearchCandidates(groups [][]CreatorSearchCandidate) []CreatorSearchCandidate {
	result := make([]CreatorSearchCandidate, 0)
	seen := map[string]bool{}
	for _, group := range groups {
		for _, candidate := range group {
			key := strings.ToLower(strings.TrimSpace(candidate.FeedURL))
			if key == "" || seen[key] {
				continue
			}
			seen[key] = true
			candidate.ExistingSourceID = 0
			result = append(result, candidate)
		}
	}
	sort.SliceStable(result, func(i, j int) bool {
		left, right := result[i], result[j]
		if creatorMatchPriority(left.Match) != creatorMatchPriority(right.Match) {
			return creatorMatchPriority(left.Match) < creatorMatchPriority(right.Match)
		}
		if creatorSearchPlatformPriorities[left.Platform] != creatorSearchPlatformPriorities[right.Platform] {
			return creatorSearchPlatformPriorities[left.Platform] < creatorSearchPlatformPriorities[right.Platform]
		}
		return strings.ToLower(left.Name) < strings.ToLower(right.Name)
	})
	return result
}

func creatorMatchPriority(match string) int {
	switch match {
	case "exact":
		return 0
	case "prefix":
		return 1
	case "contains":
		return 2
	case "handle":
		return 3
	default:
		return 4
	}
}

func creatorMatchKind(query, name string) string {
	normalizedQuery := normalizeBestBlogsText(query)
	normalizedName := normalizeBestBlogsText(name)
	if normalizedQuery == "" || normalizedName == "" {
		return "search"
	}
	switch {
	case normalizedName == normalizedQuery:
		return "exact"
	case strings.HasPrefix(normalizedName, normalizedQuery):
		return "prefix"
	case strings.Contains(normalizedName, normalizedQuery):
		return "contains"
	default:
		return "search"
	}
}

func creatorHandleMatch(query, handle string) string {
	normalizedQuery := strings.ToLower(strings.TrimPrefix(strings.TrimSpace(query), "@"))
	normalizedHandle := strings.ToLower(strings.TrimSpace(handle))
	switch {
	case normalizedQuery == "" || normalizedHandle == "":
		return ""
	case normalizedHandle == normalizedQuery:
		return "exact"
	case strings.HasPrefix(normalizedHandle, normalizedQuery):
		return "prefix"
	case strings.Contains(normalizedHandle, normalizedQuery):
		return "contains"
	default:
		return ""
	}
}

func (a *App) searchAppleCreators(ctx context.Context, query string) ([]CreatorSearchCandidate, error) {
	base := strings.TrimRight(strings.TrimSpace(a.appleAPIBase), "/")
	if base == "" {
		base = "https://itunes.apple.com"
	}
	endpoint := base + "/search?term=" + url.QueryEscape(query) + "&entity=podcast&limit=" + fmt.Sprint(creatorSearchAppleLimit)
	headers := http.Header{}
	headers.Set("Accept", "application/json")
	response, err := a.fetcher.GetBytes(ctx, endpoint, 2<<20, headers)
	if err != nil {
		return nil, fmt.Errorf("Apple Podcasts 搜索失败：%v", err)
	}
	var payload appleSearchResponse
	if err := json.Unmarshal(response.Body, &payload); err != nil {
		return nil, fmt.Errorf("Apple Podcasts 返回的数据无法解析")
	}
	result := make([]CreatorSearchCandidate, 0, len(payload.Results))
	seen := map[string]bool{}
	for _, item := range payload.Results {
		feedURL := strings.TrimSpace(item.FeedURL)
		if feedURL == "" || seen[feedURL] {
			continue
		}
		if _, err := parseHTTPURL(feedURL); err != nil {
			continue
		}
		name := strings.TrimSpace(item.CollectionName)
		if name == "" {
			name = strings.TrimSpace(item.TrackName)
		}
		if name == "" {
			name = "Apple Podcasts"
		}
		icon := strings.TrimSpace(item.ArtworkURL600)
		if icon == "" {
			icon = strings.TrimSpace(item.ArtworkURL100)
		}
		note := "Apple Podcasts 目录"
		if artist := strings.TrimSpace(item.ArtistName); artist != "" {
			note += " · " + artist
		}
		seen[feedURL] = true
		result = append(result, CreatorSearchCandidate{
			Platform: "apple",
			Kind:     "rss",
			Name:     name,
			FeedURL:  feedURL,
			SiteURL:  strings.TrimSpace(item.CollectionViewURL),
			IconURL:  icon,
			Note:     note,
			Match:    creatorMatchKind(query, name),
			Source:   "native",
		})
	}
	return result, nil
}

func (a *App) searchBlueskyCreators(ctx context.Context, query string) ([]CreatorSearchCandidate, error) {
	base := strings.TrimRight(strings.TrimSpace(a.bskyAppViewBase), "/")
	if base == "" {
		base = creatorSearchDefaultBskyBase
	}
	endpoint := base + "/xrpc/app.bsky.actor.searchActors?q=" + url.QueryEscape(query) + "&limit=" + fmt.Sprint(creatorSearchBlueskyLimit)
	headers := http.Header{}
	headers.Set("Accept", "application/json")
	response, err := a.fetcher.GetBytes(ctx, endpoint, 2<<20, headers)
	if err != nil {
		return nil, fmt.Errorf("Bluesky 搜索失败：%v", err)
	}
	var payload bskyActorSearchResponse
	if err := json.Unmarshal(response.Body, &payload); err != nil {
		return nil, fmt.Errorf("Bluesky 返回的数据无法解析")
	}
	result := make([]CreatorSearchCandidate, 0, len(payload.Actors))
	seen := map[string]bool{}
	for _, actor := range payload.Actors {
		handle := strings.TrimSpace(actor.Handle)
		if handle == "" || seen[handle] {
			continue
		}
		name := strings.TrimSpace(actor.DisplayName)
		if name == "" {
			name = handle
		}
		match := creatorMatchKind(query, name)
		if handleMatch := creatorHandleMatch(query, handle); handleMatch != "" && (match == "search" || creatorMatchPriority(handleMatch) < creatorMatchPriority(match)) {
			match = handleMatch
		}
		seen[handle] = true
		result = append(result, CreatorSearchCandidate{
			Platform: "bluesky",
			Kind:     "rss",
			Name:     name,
			Handle:   handle,
			FeedURL:  "https://bsky.app/profile/" + url.PathEscape(handle) + "/rss",
			SiteURL:  "https://bsky.app/profile/" + url.PathEscape(handle),
			IconURL:  strings.TrimSpace(actor.Avatar),
			Note:     "Bluesky 官方 RSS",
			Match:    match,
			Source:   "native",
		})
	}
	return result, nil
}

func (a *App) searchBestBlogsCreators(ctx context.Context, query string) ([]CreatorSearchCandidate, error) {
	candidates, err := a.searchBestBlogs(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("BestBlogs 目录搜索失败：%v", err)
	}
	result := make([]CreatorSearchCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		result = append(result, CreatorSearchCandidate{
			Platform: candidate.Platform,
			Kind:     candidate.Kind,
			Name:     candidate.Name,
			FeedURL:  candidate.FeedURL,
			SiteURL:  candidate.SiteURL,
			IconURL:  candidate.IconURL,
			Note:     candidate.Note,
			Match:    creatorMatchKind(query, candidate.Name),
			Source:   "bestblogs",
			Partial:  candidate.Partial,
		})
	}
	return result, nil
}

func (a *App) markExistingCreatorSources(ctx context.Context, candidates []CreatorSearchCandidate) {
	if len(candidates) == 0 || a.db == nil {
		return
	}
	rows, err := a.db.QueryContext(ctx, `SELECT id, feed_url FROM sources WHERE feed_url <> ''`)
	if err != nil {
		a.logger.Warn("creator search existing source lookup failed", "err", err)
		return
	}
	defer rows.Close()
	existing := map[string]int64{}
	for rows.Next() {
		var id int64
		var feedURL string
		if err := rows.Scan(&id, &feedURL); err != nil {
			a.logger.Warn("creator search existing source scan failed", "err", err)
			return
		}
		key := strings.TrimSpace(feedURL)
		if key == "" {
			continue
		}
		if _, ok := existing[key]; !ok {
			existing[key] = id
		}
	}
	if err := rows.Err(); err != nil {
		a.logger.Warn("creator search existing source rows failed", "err", err)
		return
	}
	for index := range candidates {
		if id, ok := existing[strings.TrimSpace(candidates[index].FeedURL)]; ok {
			candidates[index].ExistingSourceID = id
		}
	}
}

func creatorSearchCacheKey(query string, platforms []string) string {
	ordered := append([]string(nil), platforms...)
	sort.Strings(ordered)
	return strings.ToLower(strings.TrimSpace(query)) + "|" + strings.Join(ordered, ",")
}

func (a *App) loadCreatorSearchCache(key string) (CreatorSearchResponse, bool) {
	a.creatorSearchMu.Lock()
	defer a.creatorSearchMu.Unlock()
	if a.creatorSearchCache == nil {
		return CreatorSearchResponse{}, false
	}
	entry, ok := a.creatorSearchCache[key]
	if !ok {
		return CreatorSearchResponse{}, false
	}
	if a.now().After(entry.expires) {
		delete(a.creatorSearchCache, key)
		return CreatorSearchResponse{}, false
	}
	return cloneCreatorSearchResponse(entry.response), true
}

func (a *App) storeCreatorSearchCache(key string, response CreatorSearchResponse) {
	ttl := creatorSearchCacheTTL
	for _, status := range response.Platforms {
		if status.Status == creatorSearchStatusError || status.Status == creatorSearchStatusTimeout {
			ttl = creatorSearchFailureCacheTTL
			break
		}
	}
	a.creatorSearchMu.Lock()
	defer a.creatorSearchMu.Unlock()
	if a.creatorSearchCache == nil {
		a.creatorSearchCache = map[string]creatorSearchCacheEntry{}
	}
	if len(a.creatorSearchCache) >= creatorSearchCacheMaxEntries {
		now := a.now()
		for existingKey, entry := range a.creatorSearchCache {
			if now.After(entry.expires) {
				delete(a.creatorSearchCache, existingKey)
			}
		}
		if len(a.creatorSearchCache) >= creatorSearchCacheMaxEntries {
			for existingKey := range a.creatorSearchCache {
				delete(a.creatorSearchCache, existingKey)
				break
			}
		}
	}
	stored := cloneCreatorSearchResponse(response)
	for index := range stored.Candidates {
		stored.Candidates[index].ExistingSourceID = 0
	}
	a.creatorSearchCache[key] = creatorSearchCacheEntry{
		response: stored,
		expires:  a.now().Add(ttl),
	}
}

func cloneCreatorSearchResponse(response CreatorSearchResponse) CreatorSearchResponse {
	cloned := CreatorSearchResponse{
		Query:      response.Query,
		Candidates: append([]CreatorSearchCandidate(nil), response.Candidates...),
		Platforms:  append([]CreatorSearchPlatformStatus(nil), response.Platforms...),
	}
	if cloned.Candidates == nil {
		cloned.Candidates = []CreatorSearchCandidate{}
	}
	if cloned.Platforms == nil {
		cloned.Platforms = []CreatorSearchPlatformStatus{}
	}
	return cloned
}
