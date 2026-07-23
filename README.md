# 知命 · PredictYourDestiny

AI 辅助的东方命理与西方占卜网站。已上线 13 项命理功能：八字命理、周公解梦、生肖运势、万年历、男女配对、称骨算命、抽签、梅花易数、姓名分析、占星本命盘、星座运势、塔罗牌、紫微斗数。

> 现状：**阶段 0–4 已完成**。后端 API 跑通（Gin + GORM + PostgreSQL + JWT），用户端 React SPA 完成，独立的 Admin 后台已完成。

## 技术栈

| 层 | 技术 |
|---|------|
| 用户端 | React + Vite + TypeScript + Tailwind CSS v4 + react-i18next |
| 管理端 | React + Vite + TypeScript + Tailwind CSS v4（独立 `admin/` 项目） |
| 后端 | Go + Gin + GORM |
| 命理计算 | [lunar-go](https://github.com/6tail/lunar-go)（八字/黄历/节气/生肖/星座/大运/流年） |
| AI 网关 | OpenAI 兼容协议，支持多供应商动态切换 |
| 数据库 | PostgreSQL（GORM AutoMigrate 自动建表） |
| 认证 | JWT + bcrypt |
| 国际化 | 简体中文 / 繁体中文（后续扩展） |

## 架构要点

- **三个独立项目**：
  - `backend/`：Go API 服务
  - `frontend/`：用户端 React SPA
  - `admin/`：管理端 React SPA（独立部署、独立鉴权）
- **前后端分离**：前后端可独立部署到不同域名，通过 `VITE_API_BASE_URL` / `VITE_ADMIN_API_BASE_URL` 配置 API。
- **动态配置**：除 `DATABASE_URL` 与 `JWT_SECRET` 外，AI 密钥、模型列表、供应商配置、会员层级都存在数据库，Admin 可随时修改、即时生效。
- **多供应商 AI**：Admin 可添加多个 OpenAI 兼容供应商，运行时切换；供应商密钥使用 AES-256-GCM 加密保存。
- **多层会员与成本控制**：同时支持请求次数、幂等键、模型权益、Token 用量和每日成本预算。成本不足在生成前拒绝，不会截断已经开始的回答。
- **AI 成本可观测**：按供应商/模型维护不可变价格版本，记录同步与 SSE 请求的 Token、状态、预留额和结算成本。
- **服务端可信历史**：登录用户的计算结果由服务端保存，客户端不能伪造历史记录。
- **JWT 认证**：登录后 24 小时有效，API 自动携带 token。

## 目录结构

```
PredictYourDestiny/
├── backend/                # Go API 服务
│   ├── cmd/server/         # 入口
│   ├── internal/
│   │   ├── config/         # 启动配置
│   │   ├── model/          # GORM 模型
│   │   ├── store/          # 仓储层
│   │   ├── server/         # Gin 路由
│   │   ├── handler/        # HTTP handler（含 auth/admin）
│   │   ├── fortune/        # 13 个命理计算引擎
│   │   ├── ai/             # AI 网关
│   │   └── auth/           # JWT + bcrypt + 中间件
│   └── seed/               # 种子数据（解梦/抽签/塔罗/笔画）
├── frontend/               # 用户端 React SPA
│   └── src/
│       ├── api/            # API 封装（含 auth/records/quota）
│       ├── auth/           # AuthContext
│       ├── components/     # 布局
│       ├── pages/          # 各功能页面 + 登录/注册/个人中心
│       └── i18n/           # 翻译
└── admin/                  # 管理端 React SPA（独立项目）
    └── src/
        ├── api/            # API 封装
        ├── components/     # Layout + Sidebar
        ├── pages/          # Dashboard/Users/Providers/Tiers/Usage/Settings
        └── i18n/           # 翻译
```

## OpenAI 兼容供应商配置

在 Admin 的“模型供应商”页面录入 Base URL、API Key 和模型目录。密钥不得写入代码、README 或提交到 Git。

以下兼容配置已于 2026-07-23 通过 `/v1/models` 和 `/v1/chat/completions` 实际验证：

```text
Base URL: https://api.littlelan.cn/v1
Model: qwen3.7-plus
```

模型目录示例：

```json
[
  {
    "id": "qwen3.7-plus",
    "label": "Qwen 3.7 Plus",
    "tier": "free"
  }
]
```

供应商返回 OpenAI Chat Completions 结构、`reasoning_content` 和 `usage`。正式启用成本预算前，还应在“AI 用量与成本”页面为模型创建价格版本和单请求预留额；价格应以供应商当前账单为准。

## 本地开发

### 前置

- Go ≥ 1.22
- Node.js ≥ 20
- 一个可访问的 PostgreSQL

### 环境变量

后端会按顺序读取操作系统环境变量、`backend/.env` 和 `backend/.env.local`；操作系统环境变量优先。可从示例开始：

```bash
cd backend
cp .env.example .env
```

后端变量：

| 变量 | 必需性与默认值 | 说明 |
|---|---|---|
| `DATABASE_URL` | 必需，无默认值 | PostgreSQL DSN，支持 `postgresql://...` URL 或 libpq 关键字格式。未指定数据库名或使用 `postgres` 时，应用改用 `predictdestiny`；首次建库需要连接角色具有 `CREATEDB` 权限。 |
| `JWT_SECRET` | 认证功能必需 | JWT 签名密钥。生产环境使用足够长的随机值；缺失时服务可以启动，但登录和鉴权不可用。 |
| `AI_PROVIDER_ENCRYPTION_KEY` | 使用 AI 供应商时必需 | 用于加密数据库内供应商 API Key 的固定 AES-256-GCM 主密钥。使用 `openssl rand -base64 32` 生成，部署后必须稳定保存；丢失后已有密文无法解密。 |
| `APP_ENV` | 可选，默认 `development` | 设为 `production` 时启用生产安全要求，并强制配置 CORS 白名单。 |
| `CORS_ALLOWED_ORIGINS` | 生产环境必需 | 允许访问 API 的前台和 Admin Origin，逗号分隔，例如 `https://app.example.com,https://admin.example.com`。只填写 Origin，不包含路径。 |
| `SERVER_ADDR` | 可选，默认 `:8080` | API 监听地址。 |
| `LOG_LEVEL` | 可选，默认 `info` | `debug`、`info`、`warn` 或 `error`。 |
| `HISTORY_RETENTION_DAYS` | 可选，默认 `365` | 服务端历史记录保留天数；设为 `0` 禁用自动清理。 |
| `AI_RESERVATION_RETENTION_DAYS` | 可选，默认 `30` | AI 幂等及已结算成本预留记录的保留天数；设为 `0` 禁用自动清理。 |
| `ADMIN_EMAIL` | 首次初始化可选 | 仅在数据库尚无管理员时，与 `ADMIN_PASSWORD` 一起创建首个管理员。已有管理员时不会重置账号。 |
| `ADMIN_PASSWORD` | 首次初始化可选 | 首个管理员密码，至少 12 个字符。不得提交到 Git；初始化完成后应从运行环境移除。 |

生产环境最小示例（值均为占位符）：

```dotenv
DATABASE_URL=postgresql://app_user:strong-password@db.example.com:5432/predictdestiny?sslmode=require
JWT_SECRET=replace-with-a-long-random-secret
AI_PROVIDER_ENCRYPTION_KEY=replace-with-output-of-openssl-rand-base64-32
APP_ENV=production
CORS_ALLOWED_ORIGINS=https://app.example.com,https://admin.example.com
SERVER_ADDR=:8080
```

浏览器端变量在执行 Vite 构建时写入静态资源，部署后修改运行环境不会自动改变已经构建的文件：

| 项目 | 变量 | 默认值 | 生产示例 |
|---|---|---|---|
| 用户端 | `VITE_API_BASE_URL` | 空；使用同源 `/api` | `https://api.example.com`。客户端会自动追加 `/api`。 |
| Admin | `VITE_ADMIN_API_BASE_URL` | `/api` | `https://api.example.com/api`。该值必须包含 `/api` 路径。 |

AI 供应商 Base URL、API Key、模型目录、价格版本和会员成本预算不是启动环境变量，应登录 Admin 后保存到数据库。真实 API Key、`.env`、JWT 密钥和供应商加密主密钥都不得提交到 Git。

### 后端

```bash
cd backend
cp .env.example .env        # 填入 DATABASE_URL 和 JWT_SECRET
go run ./cmd/server         # 监听 :8080
```

启动时会自动 AutoMigrate 建表并初始化默认配置（含默认会员层级）。生产环境的版本化迁移仍在路线图中。验证：

```bash
curl localhost:8080/api/health
curl localhost:8080/api/ready
```

### 用户端

```bash
cd frontend
npm install
npm run dev                 # 监听 :5173，/api 代理到 :8080
```

### 管理端

```bash
cd admin
npm install
npm run dev                 # 监听 :5174，/api 代理到 :8080
```

打开 http://localhost:5174 ，用 admin 账号登录（需在数据库手动将某用户的 role 设为 'admin'）。

### 生产构建

```bash
# 用户端
cd frontend
VITE_API_BASE_URL=https://api.yourdomain.com npm run build

# 管理端
cd admin
VITE_ADMIN_API_BASE_URL=https://api.yourdomain.com/api npm run build

# 后端
cd backend
go build -o server ./cmd/server
```

## 路线图

- [x] **阶段 0** — 脚手架与基础设施
- [x] **阶段 1** — 八字命理核心
- [x] **阶段 2** — 周公解梦 / 万年历 / 生肖 / 配对
- [x] **阶段 3** — 称骨 / 抽签 / 梅花 / 姓名 / 占星 / 紫微 / 星座 / 塔罗
- [x] **阶段 4** — JWT 认证 / 请求与成本配额 / 多供应商 AI / 多层会员 / Admin 后台
- [ ] **阶段 5** — SEO / 缓存 / 英文 i18n / 内容合规

## 免责声明

本站内容仅供娱乐与文化参考，不构成任何专业建议。
