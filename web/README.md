# tg-emoji-hub · 前端

> This guide is in Chinese. For an English overview, see the [README](../README.md).

Telegram Premium 自定义表情搜索页。技术栈：React 19 + TypeScript + Vite + Tailwind CSS 4 + shadcn/ui 风格组件（基于 Radix），状态管理用 Zustand，请求用 Axios，`.tgs` 动画用 lottie-web 渲染。界面支持中文和英文，默认中文，选择保存在浏览器 `localStorage`。

## 本地开发

```bash
# 1. 先在仓库根目录启动后端（需要可用的 MySQL，见根目录 README）
go run ./cmd/server

# 2. 启动前端开发服务器
cd web
npm ci
npm run dev   # http://127.0.0.1:5173
```

Vite 开发服务器把 `/api`、`/healthz`、`/mcp` 转发到 `http://localhost:8080`。缩略图和原文件直接使用接口返回的完整链接（以后端配置 `server.base_url` 开头），因此开发时请保持 `base_url` 为 `http://localhost:8080`。

如需让前端请求其他地址的后端，可在构建或启动前设置环境变量 `VITE_API_BASE`（例如 `https://emoji.example.com`），默认为空，即请求当前站点。

## 检查与构建

```bash
npx tsc -b --force   # 类型检查，npm run lint 执行的是同一条命令
npm run build        # 输出到 web/dist/
```

请使用 `tsc -b`，不要使用 `tsc --noEmit`：`tsconfig.json` 只包含项目引用，`--noEmit` 实际不检查任何文件。

`web/dist/` 是纯静态文件，由 Nginx 提供访问：Docker 部署中是 `web` 容器，服务器部署中是站点的 Nginx 配置（见 `deployments/` 下的部署指南）。Go 主服务本身不托管前端文件。

## 目录结构

```
src/
├── api/            后端类型定义 + Axios 客户端（字段保持 snake_case，与 docs/api.md 一致）
├── components/     业务组件（SearchBar / EmojiGrid / EmojiCard / EmojiMedia / CopyMenu /
│   │               AddPackDialog / McpDialog / PackFilter / HotPacks / StatsHero …）
│   └── ui/         基础组件（button / input / dialog / popover / badge / tabs / skeleton）
├── hooks/          useSearch / usePacks / useDebouncedValue 等
├── i18n/           中英文文案与语言切换
├── lib/            工具函数（cn、剪贴板复制等）
├── pages/          SearchPage（目前唯一页面）
├── store/          Zustand 搜索状态
└── index.css       Tailwind v4 + tw-animate-css + 主题变量
```

## 与后端接口的约定

接口字段以 [docs/api.md](../docs/api.md)（英文）为准。前端类型定义在 `src/api/types.ts`，字段名一律使用 `snake_case`，不要在 Axios 拦截器里做驼峰转换。11 个分类的取值在 `src/api/types.ts` 中定义，需要与后端 `internal/domain/entity/emoji_sticker.go` 保持一致。

依赖版本以 `package.json` 和 `package-lock.json` 为准。

## 快捷键

| 键 | 动作 |
| --- | --- |
| `/` | 聚焦搜索框 |
| `Enter`（搜索框内） | 立即查询（跳过 300ms 防抖） |
| `←` `↑` `↓` `→` | 在结果网格中移动焦点 |
| `Enter`（卡片聚焦时） | 复制 HTML 格式 |
| `Esc` | 清空搜索 / 关闭弹层 |

## 复制格式

点击任意表情卡片可选择 4 种格式：

| 格式 | 示例 |
| --- | --- |
| ID | `5404674261681232568` |
| HTML | `<tg-emoji emoji-id="5404674261681232568">🔥</tg-emoji>` |
| Go map | `"fire": "5404674261681232568",` |
| JSON | `{"id":"5404674261681232568","alt":"🔥","pack":"AnimatedEmojies"}` |

在非 HTTPS 且非 localhost 的页面上，浏览器不提供 `navigator.clipboard`，此时会自动改用 `textarea + execCommand` 方式复制。

## 添加表情包

页头「添加表情包」按钮 → 输入 Telegram 表情包的 `short_name`（链接 `https://t.me/addemoji/<short_name>` 的最后一段，例如 `MicrosoftAnimatedEmojies`）→ 后端写入信号文件，由常驻运行的爬虫（`crawler.py watch`）异步爬取。爬虫在运行时，通常 1～2 分钟内即可搜索到。
