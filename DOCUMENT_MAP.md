# 文档地图

> 本表只负责导航，不保存产品或技术结论。每项职责应指向仓库当前实际路径，并标记状态。
>
> 最近更新：2026-11-12。

## 状态约定

- `现行`：当前权威来源。
- `参考`：历史、研究、原型或辅助资料，不作为现行约束。
- `缺失`：当前没有明确事实来源，不能自行补事实。
- `不适用`：当前范围明确不需要，并在初始化记录中写明原因。

## 当前映射

| 职责 | 类型 | 状态 | 实际路径 | 读取 / 更新触发条件 |
|---|---|---|---|---|
| 文档地图 | 导航 | 现行 | [`DOCUMENT_MAP.md`](DOCUMENT_MAP.md) | 文档正本、路径或状态变化时更新 |
| 产品事实 / PRD | 产品事实 | 现行 | [`PRD.md`](PRD.md) | 产品范围、用户流程、功能优先级、权限口径、验收变化 |
| 技术事实 / PDD | 技术事实 | 现行 | [`PDD.md`](PDD.md) | 架构、数据模型、接口、抓取、安全、运行时或兼容变化 |
| 订阅源实现登记 | 技术事实 / 源登记 | 现行 | [`docs/RSS-SOURCES.md`](docs/RSS-SOURCES.md) | 新增或修改任何订阅源、补充实测、调整降级方案 |
| 公众号无登录调研 | 参考 | 参考 | [`docs/research/2026-10-wechat-no-login.md`](docs/research/2026-10-wechat-no-login.md) | 只作为排除依据和研究记录，不作为产品能力 |
| 公众号限流替代调研 | 参考 | 参考 | [`docs/research/2026-11-wechat-rate-limit-alternatives.md`](docs/research/2026-11-wechat-rate-limit-alternatives.md) | 公众号限流、微信读书/Wechat2RSS 选型；作为方案依据，不作为已验证能力 |
| 原型与视觉参考 | 参考 | 参考 | [`designs/rill-prototype/`](designs/rill-prototype/) | 需要对照 v0.1 视觉、布局和交互原型时读取；主题以 `docs/DESIGN.md` 为准 |
| 设计规范 | 设计事实 | 现行 | [`docs/DESIGN.md`](docs/DESIGN.md) | UI、交互、主题 token、组件或文案变化；v0.2 起为现行视觉与语言规范 |
| 产品用户流程 | 产品事实 | 现行 | [`PRD.md`](PRD.md) §5–§6 | 用户主流程、功能范围或验收变化 |
| 技术业务流程 | 技术事实 | 现行 | [`PDD.md`](PDD.md) §6.5、§9 | 抓取、调度、阅读、失败恢复等跨模块流程变化 |
| 前端运行与构建 | 技术事实 | 现行 | [`web/README.md`](web/README.md) | 前端开发、mock/真实 API、测试或构建变化 |
| 后端运行与构建 | 技术事实 | 现行 | [`server/README.md`](server/README.md) | 后端环境变量、运行、测试或构建变化 |
| 决策记录 | 决策 | 现行 | [`docs/DECISIONS.md`](docs/DECISIONS.md) | 外部依赖、架构、安全、公开接口或难回退方案变化 |
| 需求台账 | 交付台账 | 现行 | [`docs/REQUIREMENTS.md`](docs/REQUIREMENTS.md) | 新增或更新非 Bug 需求 |
| 详细需求记录 | 交付台账 | 现行 | [`docs/requirements/`](docs/requirements/) | 跨模块、复杂或长期需求开始/变化 |
| 项目进度 | 交付台账 | 现行 | [`docs/PROGRESS.md`](docs/PROGRESS.md) | 项目基线、里程碑、证据或下一步变化 |
| 更新日志 | 发布说明 | 现行 | [`CHANGELOG.md`](CHANGELOG.md) | 用户可见功能、行为、修复与兼容性变化；按版本记录 |
| 功能进度记录 | 交付台账 | 现行 | [`docs/progress/`](docs/progress/) | 大任务阶段、DoD、阻塞或验证证据变化 |
| Bug 台账 | 交付台账 | 现行 | [`docs/BUGS.md`](docs/BUGS.md) | 出现可复现 Bug 时登记现象、影响、根因、修复和验证；首个 BUG-001 已登记 |
| 前端验证入口 | 工程 | 现行 | [`web/package.json`](web/package.json)、`web/src/**/*.test.*` | 修改前端或准备交付；证据回填项目进度 |
| 后端验证入口 | 工程 | 现行 | [`server/go.mod`](server/go.mod)、`server/internal/**/*_test.go` | 修改后端或准备交付；证据回填项目进度 |

## 初始化记录

- 最近审计日期：2026-11-01
- 沿用的既有规则/文档：`README.md`、`PRD.md`、`PDD.md`、`docs/RSS-SOURCES.md`、`docs/research/2026-10-wechat-no-login.md`、`web/README.md`、`server/README.md`、`designs/rill-prototype/`。
- 新建的缺失职责：`DOCUMENT_MAP.md`、`docs/DECISIONS.md`、`docs/REQUIREMENTS.md`、`docs/PROGRESS.md`、`docs/requirements/REQ-001..003`、`docs/requirements/REQ-019`、`docs/requirements/REQ-020`、`docs/progress/PROG-REQ-001..002`、`docs/progress/PROG-REQ-019`。
- 2026-11-01 新增现行文档：LPK V2 清单/脚本、懒猫 OIDC 设计（PDD §12.3/§13）、REQ-019 与 PROG-REQ-019、REQ-020（阅读与源管理 UI 细化）。
- 参考/历史资料：原型目录、公众号无登录调研、旧设计截图；不作为现行能力依据。
- 待澄清冲突：
  - `PRD.md` §11 的 M0–M6 仍全部标「待批准」，与当前已有实现不一致；PRD 保留为产品计划，实际交付状态以 `docs/PROGRESS.md` 为准，不据此改写产品范围。
  - `docs/RSS-SOURCES.md` 表中的「已验证」是先前外部调研结论，不等于当前 `server/` 端到端已通过；文首已加范围说明。
  - 全站聚合 Feed/OPML 完整 token 已按 REQ-004 / DEC-010 的 CSRF reveal 端点实现，普通设置仍只返回 masked。
  - SchemaVersion 3：迁移 2 增加每源保留字段，迁移 3 增加 FTS5 trigram；已在 PDD v0.8 补记。后续 schema 变更仍先走决策关卡。
- 不适用职责：
  - 多用户与 Rill 自有账号体系：只接入懒猫 OIDC，不建用户表、角色或邀请。
  - 已不适用：LPK 构建文档；2026-11-01 用户确认开始打包，见 DEC-012。
