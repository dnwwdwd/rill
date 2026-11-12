# Rill 交互原型

基于 [PRD.md](../../PRD.md) 与 [PDD.md](../../PDD.md) 制作的响应式交互原型，覆盖登录、阅读、详情、源管理、源诊断、设置 6 个页面，桌面与手机共用同一套界面。

## 预览

多文件原型需要通过 HTTP 打开，直接双击 `index.html` 不会加载 `.jsx` 组件。

```bash
# 在仓库根目录
python3 -m http.server 4311 --directory designs
```

打开：<http://localhost:4311/rill-prototype/index.html>

原型栏可以切换 6 个页面与「桌面 / 手机」视图。登录页输入任意非空密码即可进入。

## 文件

| 文件 | 内容 |
| --- | --- |
| `index.html` | 入口；本地 React / ReactDOM / Babel 与样式 |
| `app.jsx` | 路由状态、登录、核心交互 |
| `reading.jsx` | 列表、阅读栏、专注阅读、AI 摘要与翻译 |
| `sources.jsx` | 源管理、添加源、源诊断、编辑源 |
| `settings.jsx` | 阅读、AI、公众号、备份、安全 |
| `shell.jsx` | 原型栏、侧边栏、移动导航、抽屉 |
| `ui.jsx` / `icons.jsx` | 通用组件与图标 |
| `data.jsx` | 演示数据（不含真实凭据） |
| `styles.css` | 设计令牌、响应式布局与主题 |
| `设计说明.md` | 设计判断、令牌、页面映射与原型边界 |
| `screenshots/` | 桌面 / 手机关键页面截图 |

## 原型边界

- 所有数据为虚构演示数据；不连接真实后端，不发起外部网络请求。
- 二维码为占位图形，点击可模拟扫码成功。
- 图片使用带标注的占位块，不冒充真实媒体缓存结果。
- 不包含真实 API Key、Cookie、OAuth token 或 Feed token。
