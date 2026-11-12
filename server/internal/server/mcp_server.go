package server

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"rill/internal/version"
)

// REQ-038（DEC-029）：MCP Streamable HTTP（SSE）服务。
// 端点 /mcp 由 Bearer Token 保护；工具覆盖读取与写入，但不提供任何删除能力，
// 也不暴露设置、密钥、备份恢复等敏感写操作。

type mcpToolDef struct {
	name        string
	description string
	schema      map[string]any
	call        func(context.Context, map[string]any) (any, error)
}

func mcpObject(props map[string]any, required ...string) map[string]any {
	schema := map[string]any{
		"type":                 "object",
		"properties":           props,
		"additionalProperties": false,
	}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
}

func mcpProp(kind, description string) map[string]any {
	return map[string]any{"type": kind, "description": description}
}

func mcpArrayProp(description string, item map[string]any) map[string]any {
	return map[string]any{"type": "array", "description": description, "items": item}
}

func mcpEnumProp(description string, values ...string) map[string]any {
	items := make([]any, len(values))
	for i, value := range values {
		items[i] = value
	}
	return map[string]any{"type": "string", "description": description, "enum": items}
}

func mcpAnyProp(description string) map[string]any {
	return map[string]any{"description": description}
}

func (a *App) mcpHTTPHandler() http.Handler {
	a.mcpOnce.Do(func() {
		server := mcp.NewServer(&mcp.Implementation{
			Name:    "rill",
			Title:   "Rill",
			Version: version.Version,
		}, nil)
		for _, tool := range a.mcpTools() {
			tool := tool
			server.AddTool(&mcp.Tool{
				Name:        tool.name,
				Description: tool.description,
				InputSchema: tool.schema,
			}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				args := map[string]any{}
				if req != nil && len(req.Params.Arguments) > 0 {
					if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
						return mcpToolError(fmt.Errorf("参数不是合法 JSON 对象")), nil
					}
				}
				value, err := tool.call(ctx, args)
				if err != nil {
					return mcpToolError(err), nil
				}
				return mcpJSONResult(value)
			})
		}
		a.mcpHTTP = mcp.NewStreamableHTTPHandler(
			func(*http.Request) *mcp.Server { return server },
			&mcp.StreamableHTTPOptions{Stateless: true, JSONResponse: false},
		)
	})
	return a.mcpHTTP
}

func mcpJSONResult(value any) (*mcp.CallToolResult, error) {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return mcpToolError(fmt.Errorf("序列化工具结果失败: %w", err)), nil
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: string(data)}},
	}, nil
}

func mcpToolError(err error) *mcp.CallToolResult {
	message := err.Error()
	if message == "" {
		message = "操作失败"
	}
	return &mcp.CallToolResult{
		IsError: true,
		Content: []mcp.Content{&mcp.TextContent{Text: message}},
	}
}

func (a *App) mcpCallAPI(ctx context.Context, handler func(http.ResponseWriter, *http.Request) error, method, target, id string, body any) (any, error) {
	var reader *bytes.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("参数序列化失败: %w", err)
		}
		reader = bytes.NewReader(data)
	} else {
		reader = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, target, reader).WithContext(ctx)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if id != "" {
		req.SetPathValue("id", id)
	}
	recorder := httptest.NewRecorder()
	if err := handler(recorder, req); err != nil {
		var apiErr *apiError
		if errors.As(err, &apiErr) && apiErr.Message != "" {
			return nil, errors.New(apiErr.Message)
		}
		return nil, err
	}
	if recorder.Code >= http.StatusBadRequest {
		var payload struct {
			Error struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		if json.Unmarshal(recorder.Body.Bytes(), &payload) == nil && payload.Error.Message != "" {
			return nil, fmt.Errorf("%s: %s", payload.Error.Code, payload.Error.Message)
		}
		return nil, fmt.Errorf("HTTP %d", recorder.Code)
	}
	if recorder.Body.Len() == 0 {
		return map[string]any{"ok": true}, nil
	}
	var value any
	if err := json.Unmarshal(recorder.Body.Bytes(), &value); err != nil {
		return recorder.Body.String(), nil
	}
	return value, nil
}

func mcpIntList(args map[string]any, key string) ([]int64, error) {
	raw, ok := args[key]
	if !ok {
		return nil, fmt.Errorf("缺少 %s", key)
	}
	values, ok := raw.([]any)
	if !ok || len(values) == 0 {
		return nil, fmt.Errorf("%s 必须是非空数组", key)
	}
	result := make([]int64, 0, len(values))
	for _, value := range values {
		switch typed := value.(type) {
		case float64:
			if typed <= 0 || typed != float64(int64(typed)) {
				return nil, fmt.Errorf("%s 包含无效 ID", key)
			}
			result = append(result, int64(typed))
		case json.Number:
			n, err := typed.Int64()
			if err != nil || n <= 0 {
				return nil, fmt.Errorf("%s 包含无效 ID", key)
			}
			result = append(result, n)
		default:
			return nil, fmt.Errorf("%s 必须是整数数组", key)
		}
	}
	return result, nil
}

func mcpID(args map[string]any) (string, error) {
	value, ok := args["id"]
	if !ok {
		return "", errors.New("缺少 id")
	}
	switch typed := value.(type) {
	case float64:
		if typed <= 0 || typed != float64(int64(typed)) {
			return "", errors.New("id 必须是正整数")
		}
		return fmt.Sprintf("%d", int64(typed)), nil
	case json.Number:
		if n, err := typed.Int64(); err == nil && n > 0 {
			return fmt.Sprintf("%d", n), nil
		}
	case string:
		if typed != "" {
			return typed, nil
		}
	}
	return "", errors.New("id 必须是正整数")
}

func mcpQuery(args map[string]any, keys ...string) string {
	values := url.Values{}
	for _, key := range keys {
		value, ok := args[key]
		if !ok || value == nil {
			continue
		}
		switch typed := value.(type) {
		case bool:
			if typed {
				values.Set(key, "1")
			} else {
				values.Set(key, "0")
			}
		case float64:
			values.Set(key, fmt.Sprintf("%v", typed))
		default:
			text := fmt.Sprint(typed)
			if text != "" {
				values.Set(key, text)
			}
		}
	}
	if encoded := values.Encode(); encoded != "" {
		return "?" + encoded
	}
	return ""
}

func (a *App) mcpTools() []mcpToolDef {
	idProp := mcpProp("integer", "资源 ID")
	return []mcpToolDef{
		{
			name:        "list_folders",
			description: "列出订阅分组（文件夹）及其源数量。",
			schema:      mcpObject(map[string]any{}),
			call: func(ctx context.Context, _ map[string]any) (any, error) {
				return a.mcpCallAPI(ctx, a.handleListFolders, http.MethodGet, "/api/v1/folders", "", nil)
			},
		},
		{
			name:        "create_folder",
			description: "创建订阅分组。参数：name（必填）、parent_id、sort。",
			schema: mcpObject(map[string]any{
				"name":      mcpProp("string", "分组名称，最长 200 字"),
				"parent_id": mcpProp("integer", "父分组 ID，省略为根分组"),
				"sort":      mcpProp("integer", "排序值，省略时追加到末尾"),
			}, "name"),
			call: func(ctx context.Context, args map[string]any) (any, error) {
				return a.mcpCallAPI(ctx, a.handleCreateFolder, http.MethodPost, "/api/v1/folders", "", args)
			},
		},
		{
			name:        "update_folder",
			description: "重命名或移动订阅分组。参数：id（必填）、name、parent_id（传 null 移到根）、sort。",
			schema: mcpObject(map[string]any{
				"id":        idProp,
				"name":      mcpProp("string", "新名称"),
				"parent_id": mcpProp("integer", "新父分组 ID；null 表示根分组"),
				"sort":      mcpProp("integer", "新排序值"),
			}, "id"),
			call: func(ctx context.Context, args map[string]any) (any, error) {
				id, err := mcpID(args)
				if err != nil {
					return nil, err
				}
				return a.mcpCallAPI(ctx, a.handleUpdateFolder, http.MethodPatch, "/api/v1/folders/"+id, id, args)
			},
		},
		{
			name:        "list_sources",
			description: "列出订阅源，可按 folder_id 过滤。返回未读数、条目数、抓取状态与刷新间隔。",
			schema: mcpObject(map[string]any{
				"folder_id": mcpProp("integer", "只看指定分组"),
			}),
			call: func(ctx context.Context, args map[string]any) (any, error) {
				return a.mcpCallAPI(ctx, a.handleListSources, http.MethodGet, "/api/v1/sources"+mcpQuery(args, "folder_id"), "", nil)
			},
		},
		{
			name:        "resolve_source",
			description: "解析平台链接或名称，返回可订阅的候选源（标准 RSS、Apple Podcasts、小宇宙、X、Bluesky、BestBlogs 等）。参数：query（必填）。",
			schema: mcpObject(map[string]any{
				"query": mcpProp("string", "订阅链接、账号或名称"),
				"input": mcpProp("string", "兼容字段，等同 query"),
			}, "query"),
			call: func(ctx context.Context, args map[string]any) (any, error) {
				value, ok := args["query"]
				if !ok || value == nil || value == "" {
					value = args["input"]
				}
				return a.mcpCallAPI(ctx, a.handleResolveSource, http.MethodPost, "/api/v1/sources/resolve", "", map[string]any{"input": value})
			},
		},
		{
			name:        "search_creators",
			description: "按名称搜索创作者，可指定 platforms。返回候选与各平台搜索状态。",
			schema: mcpObject(map[string]any{
				"query":     mcpProp("string", "创作者名称"),
				"platforms": mcpArrayProp("平台列表，例如 apple_podcasts、bluesky、bestblogs", mcpProp("string", "平台名")),
			}, "query"),
			call: func(ctx context.Context, args map[string]any) (any, error) {
				return a.mcpCallAPI(ctx, a.handleSearchCreators, http.MethodPost, "/api/v1/sources/search", "", args)
			},
		},
		{
			name:        "create_source",
			description: "创建订阅源并立即抓取一次。参数：feed_url（必填）、name、kind、folder_id、refresh_seconds、initial_limit、backfill_mode、fallback_url。",
			schema: mcpObject(map[string]any{
				"feed_url":        mcpProp("string", "Feed 或平台地址"),
				"name":            mcpProp("string", "源名称"),
				"kind":            mcpEnumProp("源类型，默认 rss", "rss", "x", "wechat"),
				"folder_id":       mcpProp("integer", "所属分组 ID"),
				"refresh_seconds": mcpProp("integer", "刷新间隔（60–604800 秒）"),
				"initial_limit":   mcpProp("integer", "首次回溯条数（1–100）"),
				"backfill_mode":   mcpEnumProp("抓取范围", "window", "full"),
				"fallback_url":    mcpProp("string", "备用 Feed 地址"),
				"full_text":       mcpProp("boolean", "是否抓取原文全文"),
			}, "feed_url"),
			call: func(ctx context.Context, args map[string]any) (any, error) {
				return a.mcpCallAPI(ctx, a.handleCreateSource, http.MethodPost, "/api/v1/sources", "", args)
			},
		},
		{
			name:        "update_source",
			description: "修改订阅源：名称、分组、启停、刷新间隔、保留策略、备用源或抓取范围。不删除源。",
			schema: mcpObject(map[string]any{
				"id":                  idProp,
				"name":                mcpProp("string", "新名称"),
				"folder_id":           mcpProp("integer", "新分组 ID；null 表示未分组"),
				"enabled":             mcpProp("boolean", "是否启用"),
				"refresh_seconds":     mcpProp("integer", "刷新间隔（60–604800 秒）"),
				"retention_days":      mcpProp("integer", "保留天数，0 表示不限"),
				"retention_max_items": mcpProp("integer", "保留条数，0 表示不限"),
				"backfill_mode":       mcpEnumProp("抓取范围", "window", "full"),
				"fallback_url":        mcpProp("string", "备用 Feed 地址；null 清除"),
				"note":                mcpProp("string", "源备注"),
				"full_text":           mcpProp("boolean", "是否抓取原文全文"),
			}, "id"),
			call: func(ctx context.Context, args map[string]any) (any, error) {
				id, err := mcpID(args)
				if err != nil {
					return nil, err
				}
				return a.mcpCallAPI(ctx, a.handleUpdateSource, http.MethodPatch, "/api/v1/sources/"+id, id, args)
			},
		},
		{
			name:        "move_source",
			description: "把一个订阅源移动到分组；folder_id 传 null 或 0 表示移到未分组。",
			schema: mcpObject(map[string]any{
				"id":        idProp,
				"folder_id": mcpAnyProp("目标分组 ID；null/0 表示未分组"),
			}, "id", "folder_id"),
			call: func(ctx context.Context, args map[string]any) (any, error) {
				id, err := mcpID(args)
				if err != nil {
					return nil, err
				}
				folder, _ := args["folder_id"].(float64)
				body := map[string]any{"folder_id": nil}
				if folder > 0 {
					body["folder_id"] = int64(folder)
				}
				return a.mcpCallAPI(ctx, a.handleUpdateSource, http.MethodPatch, "/api/v1/sources/"+id, id, body)
			},
		},
		{
			name:        "move_sources",
			description: "批量移动订阅源到同一分组；folder_id 传 null 或 0 表示移到未分组。",
			schema: mcpObject(map[string]any{
				"ids":       mcpArrayProp("订阅源 ID 列表", mcpProp("integer", "订阅源 ID")),
				"folder_id": mcpAnyProp("目标分组 ID；null/0 表示未分组"),
			}, "ids", "folder_id"),
			call: func(ctx context.Context, args map[string]any) (any, error) {
				ids, err := mcpIntList(args, "ids")
				if err != nil {
					return nil, err
				}
				folder, _ := args["folder_id"].(float64)
				body := map[string]any{"ids": ids, "folder_id": 0}
				if folder > 0 {
					body["folder_id"] = int64(folder)
				}
				return a.mcpCallAPI(ctx, a.handleBatchUpdateSources, http.MethodPost, "/api/v1/sources/batch-update", "", body)
			},
		},
		{
			name:        "reorder_folders",
			description: "按 ordered_ids 的顺序重排同一父分组下的子分组；parent_id 传 null 表示根分组。不会删除分组。",
			schema: mcpObject(map[string]any{
				"parent_id":   mcpAnyProp("父分组 ID；null 表示根分组"),
				"ordered_ids": mcpArrayProp("按目标顺序排列的分组 ID", mcpProp("integer", "分组 ID")),
			}, "ordered_ids"),
			call: func(ctx context.Context, args map[string]any) (any, error) {
				ids, err := mcpIntList(args, "ordered_ids")
				if err != nil {
					return nil, err
				}
				var parent any
				if value, ok := args["parent_id"].(float64); ok && value > 0 {
					parent = int64(value)
				}
				results := make([]any, 0, len(ids))
				for index, id := range ids {
					body := map[string]any{"parent_id": parent, "sort": (index + 1) * 10}
					result, err := a.mcpCallAPI(ctx, a.handleUpdateFolder, http.MethodPatch,
						"/api/v1/folders/"+strconv.FormatInt(id, 10), strconv.FormatInt(id, 10), body)
					if err != nil {
						return nil, err
					}
					results = append(results, result)
				}
				return map[string]any{"updated": len(results), "items": results}, nil
			},
		},
		{
			name:        "fetch_source",
			description: "立即抓取指定订阅源。",
			schema: mcpObject(map[string]any{
				"id": idProp,
			}, "id"),
			call: func(ctx context.Context, args map[string]any) (any, error) {
				id, err := mcpID(args)
				if err != nil {
					return nil, err
				}
				return a.mcpCallAPI(ctx, a.handleFetchSource, http.MethodPost, "/api/v1/sources/"+id+"/fetch", id, nil)
			},
		},
		{
			name:        "switch_source_fallback",
			description: "把订阅源切换到备用 Feed。",
			schema: mcpObject(map[string]any{
				"id": idProp,
			}, "id"),
			call: func(ctx context.Context, args map[string]any) (any, error) {
				id, err := mcpID(args)
				if err != nil {
					return nil, err
				}
				return a.mcpCallAPI(ctx, a.handleFallbackSource, http.MethodPost, "/api/v1/sources/"+id+"/fallback", id, nil)
			},
		},
		{
			name:        "reset_source_feed_token",
			description: "重置订阅源的公开 Feed 地址 token（旧地址立即失效）。",
			schema: mcpObject(map[string]any{
				"id": idProp,
			}, "id"),
			call: func(ctx context.Context, args map[string]any) (any, error) {
				id, err := mcpID(args)
				if err != nil {
					return nil, err
				}
				return a.mcpCallAPI(ctx, a.handleResetSourceToken, http.MethodPost, "/api/v1/sources/"+id+"/token/reset", id, nil)
			},
		},
		{
			name:        "create_wechat_source",
			description: "从微信读书已收录的公众号创建订阅源。参数：account（fakeid）、name、avatar、folder_id。",
			schema: mcpObject(map[string]any{
				"account":   mcpProp("string", "公众号 fakeid"),
				"name":      mcpProp("string", "公众号名称"),
				"avatar":    mcpProp("string", "头像 URL"),
				"folder_id": mcpProp("integer", "所属分组 ID"),
			}, "account"),
			call: func(ctx context.Context, args map[string]any) (any, error) {
				return a.mcpCallAPI(ctx, a.handleWechatCreateSource, http.MethodPost, "/api/v1/wechat/sources", "", args)
			},
		},
		{
			name:        "list_items",
			description: "列出文章。支持 folder_id、source_id、unread、read、starred、hidden、q、sort、limit、cursor。默认排除已过滤隐藏条目。",
			schema: mcpObject(map[string]any{
				"folder_id": mcpProp("integer", "分组 ID"),
				"source_id": mcpProp("integer", "订阅源 ID"),
				"unread":    mcpProp("boolean", "只看未读"),
				"read":      mcpProp("boolean", "只看已读"),
				"starred":   mcpProp("boolean", "只看收藏"),
				"hidden":    mcpProp("boolean", "只看被过滤隐藏的条目"),
				"q":         mcpProp("string", "搜索标题、正文、作者或源名称"),
				"sort":      mcpEnumProp("排序方式", "published", "fetched"),
				"limit":     mcpProp("integer", "每页条数，最大 50"),
				"cursor":    mcpProp("string", "分页游标"),
			}),
			call: func(ctx context.Context, args map[string]any) (any, error) {
				return a.mcpCallAPI(ctx, a.handleListItems, http.MethodGet,
					"/api/v1/items"+mcpQuery(args, "folder_id", "source_id", "unread", "read", "starred", "hidden", "q", "sort", "limit", "cursor"), "", nil)
			},
		},
		{
			name:        "get_item",
			description: "读取单篇文章，返回元数据与原文 Markdown 正文。",
			schema: mcpObject(map[string]any{
				"id": idProp,
			}, "id"),
			call: func(ctx context.Context, args map[string]any) (any, error) {
				id, err := mcpID(args)
				if err != nil {
					return nil, err
				}
				return a.mcpGetItem(ctx, id)
			},
		},
		{
			name:        "update_item_state",
			description: "修改单篇文章的已读、收藏或隐藏状态。",
			schema: mcpObject(map[string]any{
				"id":      idProp,
				"read":    mcpProp("boolean", "已读状态"),
				"starred": mcpProp("boolean", "收藏状态"),
				"hidden":  mcpProp("boolean", "隐藏状态"),
			}, "id"),
			call: func(ctx context.Context, args map[string]any) (any, error) {
				id, err := mcpID(args)
				if err != nil {
					return nil, err
				}
				return a.mcpCallAPI(ctx, a.handlePatchItemState, http.MethodPatch, "/api/v1/items/"+id+"/state", id, args)
			},
		},
		{
			name:        "save_podcast_progress",
			description: "保存播客播放位置。参数：id、position_seconds、duration_seconds。",
			schema: mcpObject(map[string]any{
				"id":               idProp,
				"position_seconds": mcpProp("number", "当前播放位置（秒）"),
				"duration_seconds": mcpProp("number", "总时长（秒）"),
			}, "id", "position_seconds"),
			call: func(ctx context.Context, args map[string]any) (any, error) {
				id, err := mcpID(args)
				if err != nil {
					return nil, err
				}
				return a.mcpCallAPI(ctx, a.handlePodcastProgress, http.MethodPut, "/api/v1/items/"+id+"/podcast-progress", id, args)
			},
		},
		{
			name:        "generate_summary",
			description: "为文章生成/刷新 AI 摘要（需要 AI 已启用，使用当前 AI 配额）。",
			schema: mcpObject(map[string]any{
				"id": idProp,
			}, "id"),
			call: func(ctx context.Context, args map[string]any) (any, error) {
				id, err := mcpID(args)
				if err != nil {
					return nil, err
				}
				return a.mcpCallAPI(ctx, a.handleGenerateSummary, http.MethodPost, "/api/v1/items/"+id+"/summary", id, nil)
			},
		},
		{
			name:        "translate_item",
			description: "为文章生成/刷新 AI 译文（需要 AI 已启用，使用当前 AI 配额）。",
			schema: mcpObject(map[string]any{
				"id": idProp,
			}, "id"),
			call: func(ctx context.Context, args map[string]any) (any, error) {
				id, err := mcpID(args)
				if err != nil {
					return nil, err
				}
				return a.mcpCallAPI(ctx, a.handleTranslateItem, http.MethodPost, "/api/v1/items/"+id+"/translation", id, nil)
			},
		},
		{
			name:        "mark_all_read",
			description: "把全部、指定源或指定分组下的未读文章标为已读。",
			schema: mcpObject(map[string]any{
				"source_id": mcpProp("integer", "只处理该源"),
				"folder_id": mcpProp("integer", "只处理该分组（source_id 优先）"),
			}),
			call: func(ctx context.Context, args map[string]any) (any, error) {
				return a.mcpCallAPI(ctx, a.handleMarkAllRead, http.MethodPost, "/api/v1/items/mark-all-read", "", args)
			},
		},
		{
			name:        "get_stats",
			description: "读取文章总数、未读、已读、收藏与已过滤隐藏数量。",
			schema:      mcpObject(map[string]any{}),
			call: func(ctx context.Context, _ map[string]any) (any, error) {
				return a.mcpCallAPI(ctx, a.handleItemStats, http.MethodGet, "/api/v1/items/stats", "", nil)
			},
		},
		{
			name:        "run_backup",
			description: "立即创建一次数据库备份（写入应用数据目录）。",
			schema:      mcpObject(map[string]any{}),
			call: func(ctx context.Context, _ map[string]any) (any, error) {
				return a.mcpCallAPI(ctx, a.handleBackup, http.MethodPost, "/api/v1/backup", "", nil)
			},
		},
		{
			name:        "list_filter_rules",
			description: "列出关键字/正则过滤规则。",
			schema:      mcpObject(map[string]any{}),
			call: func(ctx context.Context, _ map[string]any) (any, error) {
				return a.mcpCallAPI(ctx, a.handleListFilterRules, http.MethodGet, "/api/v1/filters/rules", "", nil)
			},
		},
		{
			name:        "create_filter_rule",
			description: "创建过滤规则。参数：name、pattern、is_regex、match_field（title/content/any）、action（mark_read/hide）、scope（源 ID 或 folder:<id> 数组）、enabled。",
			schema: mcpObject(map[string]any{
				"name":        mcpProp("string", "规则名称"),
				"pattern":     mcpProp("string", "关键字或正则"),
				"is_regex":    mcpProp("boolean", "是否正则"),
				"match_field": mcpEnumProp("匹配字段", "title", "content", "any"),
				"action":      mcpEnumProp("命中动作", "mark_read", "hide"),
				"scope":       mcpArrayProp("范围：源 ID 或 folder:<id>；空数组表示全部源", mcpAnyProp("源 ID 或 folder:<id>")),
				"enabled":     mcpProp("boolean", "是否启用"),
			}, "name", "pattern", "match_field", "action"),
			call: func(ctx context.Context, args map[string]any) (any, error) {
				return a.mcpCallAPI(ctx, a.handleCreateFilterRule, http.MethodPost, "/api/v1/filters/rules", "", args)
			},
		},
		{
			name:        "update_filter_rule",
			description: "修改过滤规则；需传完整规则字段（name、pattern、match_field、action、scope），另需 id。",
			schema: mcpObject(map[string]any{
				"id":          idProp,
				"name":        mcpProp("string", "规则名称"),
				"pattern":     mcpProp("string", "关键字或正则"),
				"is_regex":    mcpProp("boolean", "是否正则"),
				"match_field": mcpEnumProp("匹配字段", "title", "content", "any"),
				"action":      mcpEnumProp("命中动作", "mark_read", "hide"),
				"scope":       mcpArrayProp("范围：源 ID 或 folder:<id>", mcpAnyProp("源 ID 或 folder:<id>")),
				"enabled":     mcpProp("boolean", "是否启用"),
			}, "id", "name", "pattern", "match_field", "action"),
			call: func(ctx context.Context, args map[string]any) (any, error) {
				id, err := mcpID(args)
				if err != nil {
					return nil, err
				}
				return a.mcpCallAPI(ctx, a.handleUpdateFilterRule, http.MethodPut, "/api/v1/filters/rules/"+id, id, args)
			},
		},
		{
			name:        "apply_filter_rule",
			description: "把过滤规则应用到存量文章。",
			schema: mcpObject(map[string]any{
				"id": idProp,
			}, "id"),
			call: func(ctx context.Context, args map[string]any) (any, error) {
				id, err := mcpID(args)
				if err != nil {
					return nil, err
				}
				return a.mcpCallAPI(ctx, a.handleApplyFilterRule, http.MethodPost, "/api/v1/filters/rules/"+id+"/apply", id, nil)
			},
		},
		{
			name:        "list_digest_rules",
			description: "列出定时报告规则。",
			schema:      mcpObject(map[string]any{}),
			call: func(ctx context.Context, _ map[string]any) (any, error) {
				return a.mcpCallAPI(ctx, a.handleListDigestRules, http.MethodGet, "/api/v1/digest/rules", "", nil)
			},
		},
		{
			name:        "create_digest_rule",
			description: "创建定时报告规则。参数：name、scope、period（daily/weekly）、run_at_local、weekday、only_unread、push_when_empty、enabled、channel_ids。",
			schema: mcpObject(map[string]any{
				"name":            mcpProp("string", "规则名称"),
				"scope":           mcpArrayProp("范围：源 ID 或 folder:<id>；空数组表示全部源", mcpAnyProp("源 ID 或 folder:<id>")),
				"period":          mcpEnumProp("周期", "daily", "weekly"),
				"run_at_local":    mcpProp("string", "本地运行时间，例如 09:00"),
				"weekday":         mcpProp("integer", "每周运行的星期（1–7）"),
				"only_unread":     mcpProp("boolean", "只汇总未读"),
				"push_when_empty": mcpProp("boolean", "空窗口也推送"),
				"enabled":         mcpProp("boolean", "是否启用"),
				"channel_ids":     mcpArrayProp("推送渠道 ID 数组", mcpProp("integer", "渠道 ID")),
			}, "name", "period", "run_at_local"),
			call: func(ctx context.Context, args map[string]any) (any, error) {
				return a.mcpCallAPI(ctx, a.handleCreateDigestRule, http.MethodPost, "/api/v1/digest/rules", "", args)
			},
		},
		{
			name:        "update_digest_rule",
			description: "修改定时报告规则；需传 name、period、run_at_local（及 scope），另需 id。",
			schema: mcpObject(map[string]any{
				"id":              idProp,
				"name":            mcpProp("string", "规则名称"),
				"scope":           mcpArrayProp("范围：源 ID 或 folder:<id>", mcpAnyProp("源 ID 或 folder:<id>")),
				"period":          mcpEnumProp("周期", "daily", "weekly"),
				"run_at_local":    mcpProp("string", "本地运行时间"),
				"weekday":         mcpProp("integer", "每周运行的星期（1–7）"),
				"only_unread":     mcpProp("boolean", "只汇总未读"),
				"push_when_empty": mcpProp("boolean", "空窗口也推送"),
				"enabled":         mcpProp("boolean", "是否启用"),
				"channel_ids":     mcpArrayProp("推送渠道 ID 数组", mcpProp("integer", "渠道 ID")),
			}, "id", "name", "period", "run_at_local"),
			call: func(ctx context.Context, args map[string]any) (any, error) {
				id, err := mcpID(args)
				if err != nil {
					return nil, err
				}
				return a.mcpCallAPI(ctx, a.handleUpdateDigestRule, http.MethodPut, "/api/v1/digest/rules/"+id, id, args)
			},
		},
		{
			name:        "run_digest_rule",
			description: "立即运行一次定时报告规则，返回生成的报告。",
			schema: mcpObject(map[string]any{
				"id": idProp,
			}, "id"),
			call: func(ctx context.Context, args map[string]any) (any, error) {
				id, err := mcpID(args)
				if err != nil {
					return nil, err
				}
				return a.mcpCallAPI(ctx, a.handleRunDigestRule, http.MethodPost, "/api/v1/digest/rules/"+id+"/run", id, nil)
			},
		},
		{
			name:        "list_digest_reports",
			description: "列出定时报告，可按 rule_id 过滤。",
			schema: mcpObject(map[string]any{
				"rule_id": mcpProp("integer", "只返回该规则的报告"),
				"limit":   mcpProp("integer", "最多返回条数"),
			}),
			call: func(ctx context.Context, args map[string]any) (any, error) {
				return a.mcpCallAPI(ctx, a.handleListDigestReports, http.MethodGet,
					"/api/v1/digest/reports"+mcpQuery(args, "rule_id", "limit"), "", nil)
			},
		},
		{
			name:        "get_digest_report",
			description: "读取单份定时报告（含 Markdown 正文）。",
			schema: mcpObject(map[string]any{
				"id": idProp,
			}, "id"),
			call: func(ctx context.Context, args map[string]any) (any, error) {
				id, err := mcpID(args)
				if err != nil {
					return nil, err
				}
				return a.mcpCallAPI(ctx, a.handleGetDigestReport, http.MethodGet, "/api/v1/digest/reports/"+id, id, nil)
			},
		},
		{
			name:        "push_digest_report",
			description: "把已有报告重新推送到规则配置的渠道。",
			schema: mcpObject(map[string]any{
				"id": idProp,
			}, "id"),
			call: func(ctx context.Context, args map[string]any) (any, error) {
				id, err := mcpID(args)
				if err != nil {
					return nil, err
				}
				return a.mcpCallAPI(ctx, a.handlePushDigestReport, http.MethodPost, "/api/v1/digest/reports/"+id+"/push", id, nil)
			},
		},
		{
			name:        "list_push_channels",
			description: "列出推送渠道（URL 已打码）。",
			schema:      mcpObject(map[string]any{}),
			call: func(ctx context.Context, _ map[string]any) (any, error) {
				return a.mcpCallAPI(ctx, a.handleListChannels, http.MethodGet, "/api/v1/channels", "", nil)
			},
		},
		{
			name:        "create_push_channel",
			description: "创建推送渠道。参数：name、kind（feishu/generic_webhook）、url、secret（可选）、enabled。",
			schema: mcpObject(map[string]any{
				"name":    mcpProp("string", "渠道名称"),
				"kind":    mcpEnumProp("渠道类型", "feishu", "generic_webhook"),
				"url":     mcpProp("string", "Webhook URL"),
				"secret":  mcpProp("string", "飞书签名校验密钥（可选）"),
				"enabled": mcpProp("boolean", "是否启用"),
			}, "name", "kind", "url"),
			call: func(ctx context.Context, args map[string]any) (any, error) {
				return a.mcpCallAPI(ctx, a.handleCreateChannel, http.MethodPost, "/api/v1/channels", "", args)
			},
		},
		{
			name:        "update_push_channel",
			description: "修改推送渠道（不删除）。参数：id、name 必填；url、secret、enabled 可选。",
			schema: mcpObject(map[string]any{
				"id":      idProp,
				"name":    mcpProp("string", "渠道名称"),
				"kind":    mcpEnumProp("渠道类型", "feishu", "generic_webhook"),
				"url":     mcpProp("string", "新 Webhook URL；省略表示不修改"),
				"secret":  mcpProp("string", "飞书签名密钥；空串清除"),
				"enabled": mcpProp("boolean", "是否启用"),
			}, "id", "name"),
			call: func(ctx context.Context, args map[string]any) (any, error) {
				id, err := mcpID(args)
				if err != nil {
					return nil, err
				}
				return a.mcpCallAPI(ctx, a.handleUpdateChannel, http.MethodPut, "/api/v1/channels/"+id, id, args)
			},
		},
		{
			name:        "test_push_channel",
			description: "向指定推送渠道发送测试消息。",
			schema: mcpObject(map[string]any{
				"id": idProp,
			}, "id"),
			call: func(ctx context.Context, args map[string]any) (any, error) {
				id, err := mcpID(args)
				if err != nil {
					return nil, err
				}
				return a.mcpCallAPI(ctx, a.handleTestChannel, http.MethodPost, "/api/v1/channels/"+id+"/test", id, nil)
			},
		},
	}
}

func (a *App) mcpGetItem(ctx context.Context, rawID string) (any, error) {
	id, err := strconv.ParseInt(rawID, 10, 64)
	if err != nil || id <= 0 {
		return nil, fmt.Errorf("id 必须是正整数")
	}
	detail, err := a.getItemDetail(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("item_not_found: 条目不存在")
	}
	if err != nil {
		return nil, err
	}
	origin, _ := ctx.Value(mcpOriginContextKey{}).(string)
	return map[string]any{
		"id":           detail.ID,
		"source_id":    detail.SourceID,
		"source":       detail.Source.Name,
		"kind":         detail.Source.Kind,
		"title":        detail.Title,
		"url":          detail.URL,
		"author":       detail.Author,
		"published_at": detail.PublishedAt,
		"read_at":      detail.ReadAt,
		"starred_at":   detail.StarredAt,
		"excerpt":      detail.Excerpt,
		"markdown":     buildItemMarkdown(detail, origin),
	}, nil
}
