import { describe, expect, it, vi } from 'vitest';
import { mockApi } from '@/api/mock/mockApi';

const login = () => mockApi.signIn?.();

describe('mockApi contract', () => {
  it('requires a session and emits the global unauthorized event', async () => {
    window.sessionStorage.clear();
    const onUnauthorized = vi.fn();
    window.addEventListener('rill:unauthorized', onUnauthorized);
    await expect(mockApi.getSources()).rejects.toMatchObject({ status: 401, code: 'unauthorized' });
    expect(onUnauthorized).toHaveBeenCalledTimes(1);
    window.removeEventListener('rill:unauthorized', onUnauthorized);
  });

  it('paginates unread items and returns undoable ids for bulk read', async () => {
    await login();
    const page = await mockApi.getItems({ unread: true, limit: 5 });
    expect(page.items).toHaveLength(5);
    expect(page.items.every((item) => item.read_at === null)).toBe(true);
    expect(page.next_cursor).toBeTruthy();

    const next = await mockApi.getItems({ unread: true, limit: 5, cursor: page.next_cursor ?? undefined });
    expect(next.items[0].id).not.toBe(page.items[0].id);

    const before = await mockApi.getReaderStats();
    const result = await mockApi.markAllRead({ source_id: 1 });
    expect(result.ids.length).toBeGreaterThan(0);
    const after = await mockApi.getReaderStats();
    expect(after.unread).toBeLessThan(before.unread);

    const restored = await mockApi.batchItemState(result.ids, { read: false });
    expect(restored.ids).toEqual(result.ids);
    expect((await mockApi.getReaderStats()).unread).toBe(before.unread);
  });

  it('returns optional cover images on summaries', async () => {
    await login();
    const page = await mockApi.getItems({ limit: 12 });
    const withCover = page.items.find((item) => item.cover_url);
    expect(withCover?.cover_url).toBe('/covers/cover-quiet-feed.svg');
    expect(page.items.some((item) => item.cover_url === null)).toBe(true);
  });

  it('keeps per-source token reset separate from the global feed token', async () => {
    await login();
    const settingsBefore = await mockApi.getSettings();
    const health = await mockApi.getSourceHealth(1);
    expect(health.feed_paths.rss).toContain('/feed/1/');
    expect(health.logs.length).toBeGreaterThan(0);

    const sourceToken = await mockApi.resetFeedToken(1);
    expect(sourceToken.token_masked).not.toBe(settingsBefore.feed.token_masked);
    expect((await mockApi.getSettings()).feed.token_masked).toBe(settingsBefore.feed.token_masked);  });

  it('generates summaries without consent gate', async () => {
    await login();

    await mockApi.updateSettings({ ai: { consent: false } });
    // 2026-11-02：外发确认门槛已移除，consent 不再拦截摘要生成。

    await mockApi.saveAiKey('sk-test-not-a-real-key');
    const settings = await mockApi.getSettings();
    expect(settings.ai.api_key_set).toBe(true);
    expect(Object.prototype.hasOwnProperty.call(settings.ai, 'api_key')).toBe(false);
  });

  it('discovers feed candidates without a network call', async () => {
    await login();
    const candidates = await mockApi.discoverFeed('example.com');
    expect(candidates.length).toBeGreaterThan(0);
    expect(candidates[0].url).toMatch(/^https:/);
  });

  it('resolves the BestBlogs WeChat catalog before the MP path', async () => {
    await login();
    const candidates = await mockApi.resolveSource('数字生命卡兹克');
    expect(candidates).toHaveLength(1);
    expect(candidates[0]).toMatchObject({
      platform: 'bestblogs',
      kind: 'rss',
      name: '数字生命卡兹克',
    });
    expect(candidates[0].feed_url).toContain('wechat2rss.bestblogs.dev/feed/');
  });

  it('resolves BestBlogs podcast, X, and YouTube catalog entries', async () => {
    await login();
    const cases = [
      ['bestblogs:科技早知道', 'bestblogs-podcast'],
      ['bestblogs:OpenAI', 'bestblogs-x'],
      ['bestblogs:Andrej Karpathy', 'bestblogs-youtube'],
    ] as const;
    for (const [input, platform] of cases) {
      const candidates = await mockApi.resolveSource(input);
      expect(candidates).toHaveLength(1);
      expect(candidates[0].platform).toBe(platform);
      expect(candidates[0].kind).toBe('rss');
    }
  });

  it('searches creators across platforms and marks existing sources', async () => {
    await login();
    const response = await mockApi.searchCreators('这集我看过');
    expect(response.candidates.map((candidate) => candidate.platform)).toEqual(['apple', 'bestblogs-podcast']);
    expect(response.platforms.map((status) => status.platform)).toEqual(['apple', 'bluesky', 'bestblogs']);
    expect(response.platforms.find((status) => status.platform === 'apple')?.status).toBe('ok');
    expect(response.platforms.find((status) => status.platform === 'bluesky')?.status).toBe('empty');

    const created = await mockApi.createSource({
      kind: 'rss',
      name: '这集我看过',
      feed_url: response.candidates[0].feed_url,
      folder_id: null,
    });
    const again = await mockApi.searchCreators('这集我看过');
    expect(again.candidates[0].existing_source_id).toBe(created.id);
  });

  it('batch creates sources and reports duplicates without aborting', async () => {
    await login();
    const result = await mockApi.createSources([
      { kind: 'rss', name: 'Creator A', feed_url: 'https://example.com/creator-a.xml', folder_id: null },
      { kind: 'rss', name: 'Creator A duplicate', feed_url: 'https://example.com/creator-a.xml', folder_id: null },
      { kind: 'rss', name: 'Creator B', feed_url: 'https://example.com/creator-b.xml', folder_id: null },
    ]);
    expect(result.created).toHaveLength(2);
    expect(result.errors).toHaveLength(1);
    expect(result.errors[0].code).toBe('source_exists');

    const again = await mockApi.createSources([
      { kind: 'rss', name: 'Creator A again', feed_url: 'https://example.com/creator-a.xml', folder_id: null },
    ]);
    expect(again.created).toHaveLength(0);
    expect(again.errors[0].code).toBe('source_exists');
  });

  it('manages push channels without exposing full webhook URLs', async () => {
    await login();
    const created = await mockApi.createChannel({
      name: '测试群机器人',
      kind: 'generic_webhook',
      url: 'https://hooks.example.com/webhook/super-secret-token-9f3c',
    });
    expect(created.id).toBeGreaterThan(0);
    const channels = await mockApi.getChannels();
    const channel = channels.find((entry) => entry.id === created.id);
    expect(channel?.url_masked).not.toContain('super-secret-token');
    expect(channel?.url_masked).toMatch(/^https:\/\/hooks\.example\.com\/webhook\/\*{4}/);

    await mockApi.updateChannel(created.id, { name: '改名后', kind: 'feishu' });
    const renamed = (await mockApi.getChannels()).find((entry) => entry.id === created.id);
    expect(renamed?.kind).toBe('feishu');

    const result = await mockApi.testChannel(created.id);
    expect(result.ok).toBe(true);

    await mockApi.deleteChannel(created.id);
    expect((await mockApi.getChannels()).some((entry) => entry.id === created.id)).toBe(false);
  });

  it('runs digest rules and pushes reports', async () => {
    await login();
    const created = await mockApi.createDigestRule({
      name: '测试周报',
      scope: ['folder:1'],
      period: 'weekly',
      run_at_local: '09:00',
      weekday: 3,
      only_unread: false,
      push_when_empty: true,
      channel_ids: [1],
    });
    const report = await mockApi.runDigestRule(created.id);
    expect(report.rule_name).toBe('测试周报');
    expect(report.status === 'ok' || report.status === 'empty').toBe(true);
    expect(report.content_md).toContain('# 测试周报');

    const reports = await mockApi.getDigestReports(created.id);
    expect(reports.some((entry) => entry.id === report.id)).toBe(true);

    const pushed = await mockApi.pushDigestReport(report.id);
    expect(pushed.push_status).toBe('ok');
    const detail = await mockApi.getDigestReport(report.id);
    expect(detail.pushed_at).toBeTruthy();

    await expect(mockApi.createDigestRule({
      name: '坏时间', scope: [], period: 'daily', run_at_local: '25:00',
      only_unread: false, push_when_empty: false,
    })).rejects.toMatchObject({ code: 'invalid_run_at' });

    await mockApi.deleteDigestRule(created.id);
    expect((await mockApi.getDigestReports(created.id)).some((entry) => entry.id === report.id)).toBe(false);
  });

  it('hides items through filter rules and restores them', async () => {
    await login();
    const statsBefore = await mockApi.getReaderStats();
    expect(statsBefore.hidden).toBeGreaterThan(0);

    const created = await mockApi.createFilterRule({
      name: '测试关键字',
      pattern: 'AI 浏览器',
      is_regex: false,
      match_field: 'title',
      action: 'hide',
      scope: [],
    });
    const applied = await mockApi.applyFilterRule(created.id);
    expect(applied.affected).toBeGreaterThan(0);
    const statsAfter = await mockApi.getReaderStats();
    expect(statsAfter.hidden).toBe(statsBefore.hidden + applied.affected);

    const hiddenPage = await mockApi.getItems({ hidden: true, limit: 50 });
    expect(hiddenPage.items.length).toBe(statsAfter.hidden);
    const visiblePage = await mockApi.getItems({ limit: 50 });
    expect(visiblePage.items.every((item) => item.title !== 'AI 浏览器大战，没人想只做「摘要」')).toBe(true);

    const restored = hiddenPage.items[0];
    await mockApi.patchItemState(restored.id, { hidden: false });
    expect((await mockApi.getReaderStats()).hidden).toBe(statsAfter.hidden - 1);

    await expect(mockApi.createFilterRule({
      name: '坏正则', pattern: '([unclosed', is_regex: true, match_field: 'title', action: 'mark_read', scope: [],
    })).rejects.toMatchObject({ code: 'invalid_regex' });

    await mockApi.deleteFilterRule(created.id);
  });
});
