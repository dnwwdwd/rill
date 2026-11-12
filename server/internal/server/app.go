package server

import (
	"context"
	"database/sql"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"

	"rill/internal/config"
	"rill/internal/database"
	"rill/internal/fetch"
	"rill/internal/secure"
)

type Options struct {
	DisableScheduler    bool
	Now                 func() time.Time
	Logger              *slog.Logger
	Fetcher             *fetch.Client
	StaticDir           string
	DisableInitialFetch bool
	DisableBackfill     bool
	XAPIBase            string // 测试可覆盖 api.fxtwitter.com
	BskyAppViewBase     string // 测试可覆盖 public.api.bsky.app
	AppleAPIBase        string // 测试可覆盖 itunes.apple.com
	XiaoyuzhouBase      string // 测试可覆盖 www.xiaoyuzhoufm.com
}

type App struct {
	cfg                          config.Config
	db                           *sql.DB
	logger                       *slog.Logger
	secret                       *secure.SecretBox
	fetcher                      *fetch.Client
	static                       string
	nowFn                        func() time.Time
	digestLocation               *time.Location
	disableInitialFetch          bool
	disableBackfill              bool
	xAPIBase                     string
	xFetchRetryDelay             time.Duration
	xFetchRetryCount             int
	bskyAppViewBase              string
	appleAPIBase                 string
	xiaoyuzhouBase               string
	bestblogsCatalogURL          string
	bestblogsCatalogBase         string
	bestblogsCatalogFallbackBase string
	bestblogsMu                  sync.Mutex
	bestblogsSources             []bestBlogsSource
	bestblogsFetchedAt           time.Time
	bestblogsGroup               singleflight.Group
	creatorSearchMu              sync.Mutex
	creatorSearchCache           map[string]creatorSearchCacheEntry

	fetchLocks sync.Map
	backupMu   sync.Mutex

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	mediaGroup singleflight.Group
	iconGroup  singleflight.Group

	schedMu            sync.RWMutex
	schedLastTick      time.Time
	schedLastRetention time.Time

	wereadLoginMu      sync.Mutex
	wereadLogin        *wereadLoginSession
	wereadCooldownLock sync.Mutex
	wereadCooldownAt   time.Time
	// 微信读书主动续期（keep-alive）：wr_rt 长时间不使用会被服务端作废
	//（实测返回 -2013 鉴权失败 + 清空 Cookie），必须像官方网页一样定期换票。
	wereadKeepAliveMu   sync.Mutex
	wereadLastKeepAlive time.Time

	backfillRunning sync.Map

	mcpOnce sync.Once
	mcpHTTP http.Handler
}

func New(ctx context.Context, cfg config.Config) (*App, error) {
	return NewWithOptions(ctx, cfg, Options{})
}

func NewWithOptions(ctx context.Context, cfg config.Config, opts Options) (*App, error) {
	if err := os.MkdirAll(cfg.DataDir, 0o750); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(cfg.CacheDir, 0o750); err != nil {
		return nil, err
	}
	dbPath := filepath.Join(cfg.DataDir, "rill.db")
	db, err := database.Open(ctx, dbPath)
	if err != nil {
		return nil, err
	}
	if err := database.Migrate(ctx, db, dbPath, filepath.Join(cfg.DataDir, "backups")); err != nil {
		_ = db.Close()
		return nil, err
	}
	secret, err := secure.LoadOrCreateKey(cfg.DataDir)
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	logger := opts.Logger
	if logger == nil {
		logger = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel(cfg.LogLevel)}))
	}
	nowFn := opts.Now
	if nowFn == nil {
		nowFn = time.Now
	}
	fetcher := opts.Fetcher
	if fetcher == nil {
		fetcher = fetch.NewClient(cfg.OutboundUserAgent)
	}
	appleAPIBase := strings.TrimRight(strings.TrimSpace(opts.AppleAPIBase), "/")
	if appleAPIBase == "" {
		appleAPIBase = "https://itunes.apple.com"
	}
	xiaoyuzhouBase := strings.TrimRight(strings.TrimSpace(opts.XiaoyuzhouBase), "/")
	if xiaoyuzhouBase == "" {
		xiaoyuzhouBase = "https://www.xiaoyuzhoufm.com"
	}
	appCtx, cancel := context.WithCancel(ctx)
	digestLocation := time.Local
	if name := strings.TrimSpace(cfg.Timezone); name != "" {
		if loc, locErr := time.LoadLocation(name); locErr == nil {
			digestLocation = loc
		} else {
			logger.Warn("digest timezone unavailable, falling back to system local", "timezone", name, "err", locErr)
		}
	}
	app := &App{
		cfg:                          cfg,
		db:                           db,
		logger:                       logger,
		secret:                       secret,
		fetcher:                      fetcher,
		nowFn:                        nowFn,
		digestLocation:               digestLocation,
		disableInitialFetch:          opts.DisableInitialFetch,
		disableBackfill:              opts.DisableBackfill,
		xAPIBase:                     opts.XAPIBase,
		xFetchRetryDelay:             10 * time.Second,
		xFetchRetryCount:             2,
		bskyAppViewBase:              opts.BskyAppViewBase,
		appleAPIBase:                 appleAPIBase,
		xiaoyuzhouBase:               xiaoyuzhouBase,
		bestblogsCatalogURL:          strings.TrimSpace(cfg.BestBlogsCatalogURL),
		bestblogsCatalogBase:         bestBlogsCatalogBase,
		bestblogsCatalogFallbackBase: bestBlogsCatalogFallbackBase,
		ctx:                          appCtx,
		cancel:                       cancel,
	}
	if err := app.bootstrapSettings(ctx); err != nil {
		cancel()
		_ = db.Close()
		return nil, err
	}
	app.repairDigestNextRuns(appCtx)
	app.cleanupSessions(ctx)
	app.static = resolveStaticDir(cfg.WebDir, opts.StaticDir)
	if !opts.DisableScheduler {
		app.wg.Add(1)
		go func() {
			defer app.wg.Done()
			app.runScheduler(appCtx)
		}()
	}
	return app, nil
}

func (a *App) now() time.Time { return a.nowFn().UTC() }

func (a *App) Close() {
	a.cancel()
	a.wg.Wait()
	if a.db != nil {
		_ = a.db.Close()
	}
}

func (a *App) Handler() http.Handler {
	return a.routes()
}

func logLevel(value string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func resolveStaticDir(explicit, option string) string {
	candidates := make([]string, 0, 4)
	if option != "" {
		candidates = append(candidates, option)
	}
	if explicit != "" {
		candidates = append(candidates, explicit)
	}
	candidates = append(candidates, "web/dist", "../web/dist")
	if executable, err := os.Executable(); err == nil {
		dir := filepath.Dir(executable)
		candidates = append(candidates, filepath.Join(dir, "web/dist"), filepath.Join(dir, "../web/dist"))
	}
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		absolute, err := filepath.Abs(candidate)
		if err != nil {
			continue
		}
		info, err := os.Stat(filepath.Join(absolute, "index.html"))
		if err == nil && !info.IsDir() {
			return absolute
		}
	}
	return ""
}
