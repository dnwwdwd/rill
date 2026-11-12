# REQ-029 播客订阅与播放进度持久化

- 状态：已完成（本地验证）
- 日期：2026-11-02
- 关联：PRD F-23、RSS-SOURCES §2.8、PDD §4/§6、DEC-019

## 目标

让标准 RSS 中的播客音频可以直接在 Rill 内播放，并把播放位置保存在微服，做到重新打开、换浏览器或换设备后继续听。

## 范围

1. 标准 RSS `enclosure` 中识别音频地址与 MIME（`audio/*` 或常见音频扩展名）。
2. 音频地址随条目保存，详情接口返回 `audio_url` / `audio_type`。
3. 正文标题下方显示播放器：播放/暂停、拖动进度、±15 秒、0.75–2× 倍速。
4. 播放位置写入服务端 `podcast_progress`，同时保留 `localStorage` 作为即时缓存。
5. 输出 Feed 保留音频附件：RSS `<enclosure>`、Atom `<link rel="enclosure">`、JSON Feed `attachments`。

## 数据

迁移 5 新增：

```sql
CREATE TABLE podcast_progress (
    item_id           INTEGER PRIMARY KEY REFERENCES items(id) ON DELETE CASCADE,
    position_seconds  REAL NOT NULL DEFAULT 0,
    duration_seconds  REAL NOT NULL DEFAULT 0,
    updated_at        TEXT NOT NULL
);
```

旧库启动时自动迁移；删除条目时进度级联清理；备份/恢复包含该表。

## API

- `GET /api/v1/items/{id}`：新增 `podcast_position_seconds`、`podcast_duration_seconds`（0 省略）。
- `PUT /api/v1/items/{id}/podcast-progress`：`{ "position_seconds": 123.4, "duration_seconds": 1800 }`，204；负数返回 400。

## 验收

- 本地 podcast Feed 可识别音频，详情返回地址与 MIME。
- 播放后写入进度；重新打开文章从上次位置继续。
- 播放结束清除本地位置并向服务端写 0。
- RSS/Atom/JSON 输出均包含音频附件。

## 已知限制

- 音频直接使用源站地址，不经 Rill 媒体代理；CORS/防盗链严格时可能无法播放。
- 进度按「条目 + 单用户」保存，不做跨用户；播放进度不参与内容哈希与翻译缓存。
- 视频 enclosure 不在本次范围。
