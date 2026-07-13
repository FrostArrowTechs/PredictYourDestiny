# 知命 · PredictYourDestiny

AI 辅助的东方命理与西方占卜网站。功能规划：八字命理、周公解梦、生肖运势、万年历黄道吉日、流年大运、男女配对/合盘、星座、塔罗，配合多语种，未来可扩展手相等。

> 现状：**阶段 0 脚手架已完成**。后端 API 已跑通（Gin + GORM + PostgreSQL），前端骨架就位（React + Vite + Tailwind + i18n），整条链路「前端 → 后端 → 数据库」已联调验证。下一步进入阶段 1（八字命理核心）。

## 技术栈

| 层 | 技术 |
|---|------|
| 前端 | React + Vite + TypeScript + Tailwind CSS v4 + react-i18next |
| 后端 | Go + Gin + GORM |
| 命理计算 | [lunar-go](https://github.com/6tail/lunar-go)（八字/黄历/节气/生肖/星座/大运/流年） |
| AI 网关 | OpenAI 兼容协议，对接 [New API](https://github.com/Calcium-Ion/new-api) 等网关 |
| 数据库 | PostgreSQL（GORM AutoMigrate 自动建表） |
| 国际化 | 简体中文 / 繁体中文（后续扩展） |

## 架构要点

- **前后端分离**：前端可独立部署到 Cloudflare Pages 等静态托管，后端是纯 API 服务。前端通过 `VITE_API_BASE_URL` 构建期变量指向 API 域名。
- **动态配置**：除 `DATABASE_URL` 等启动引导项写入 `.env` 外，AI 密钥、模型列表、每日配额等可变配置都存在数据库 `settings` 表，后台可随时修改、**即时生效无需重启**。
- **分层计费**：匿名用户可用所有"纯计算"功能（排盘/抽牌/黄历查询，零 AI 消耗）；登录后享每日免费 AI 解读额度；付费解锁更强模型与深度解析。

## 目录结构

```
PredictYourDestiny/
├── backend/                # Go API 服务
│   ├── cmd/server/         # 入口（启动 → AutoMigrate → 启动 Gin）
│   └── internal/
│       ├── config/         # 启动引导配置（.env）
│       ├── model/          # GORM 模型（AutoMigrate 源）
│       ├── store/          # 仓储层（含动态配置 SettingStore）
│       ├── server/         # Gin 路由 + 中间件
│       ├── handler/        # HTTP handler
│       ├── fortune/        # 命理计算引擎（阶段 1 起）
│       ├── ai/             # AI 网关（阶段 1 起）
│       └── auth/           # JWT 认证（阶段 4）
└── frontend/               # React SPA
    └── src/
        ├── api/            # API 调用封装
        ├── components/     # 布局/导航/语言切换
        ├── pages/          # 各功能页面（阶段 0 为占位）
        └── i18n/           # 简中/繁中翻译
```

## 本地开发

### 前置

- Go ≥ 1.22
- Node.js ≥ 20
- 一个可访问的 PostgreSQL（本地或远程）

### 后端

```bash
cd backend
cp .env.example .env        # 填入 DATABASE_URL
go run ./cmd/server         # 监听 :8080
```

启动时会自动 AutoMigrate 建表并初始化默认配置。验证：

```bash
curl localhost:8080/api/health   # {"status":"ok",...}
curl localhost:8080/api/ready    # {"status":"ready"}  ← 探活数据库
```

### 前端

```bash
cd frontend
npm install
npm run dev                 # 监听 :5173，/api 代理到 :8080
```

打开 http://localhost:5173 ，首页右下角的服务状态点应显示在线（即联调成功）。右上角可切换简中/繁中。

### 生产构建

```bash
# 前端：构建时指定 API 域名
cd frontend
VITE_API_BASE_URL=https://api.yourdomain.com npm run build
# 产物在 frontend/dist/，部署到 Cloudflare Pages 等

# 后端：单一二进制
cd backend
go build -o server ./cmd/server
```

## 路线图

- [x] **阶段 0** — 脚手架与基础设施（Gin + GORM + PG + AutoMigrate + 前端骨架 + i18n）
- [ ] **阶段 1** — 八字命理核心（四柱 + 大运 + 流年 + AI 解读，确立计算→解读范式）
- [ ] **阶段 2** — 其他中文体系（生肖、黄历、解梦、配对合盘）
- [ ] **阶段 3** — 西方体系（星座、塔罗）
- [ ] **阶段 4** — 账户、配额与付费
- [ ] **阶段 5** — SEO、缓存、流式输出、更多语言、手相等

## 免责声明

本站内容仅供娱乐与文化参考，不构成任何专业建议。
