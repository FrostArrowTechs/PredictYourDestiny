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
- **多供应商 AI**：Admin 可添加多个 OpenAI 兼容供应商，运行时切换。
- **多层会员体系**：免费（5次/日）、基础（20次/日）、高级（无限）。
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
        ├── pages/          # Dashboard/Users/Providers/Tiers/Settings
        └── i18n/           # 翻译
```

## 本地开发

### 前置

- Go ≥ 1.22
- Node.js ≥ 20
- 一个可访问的 PostgreSQL

### 后端

```bash
cd backend
cp .env.example .env        # 填入 DATABASE_URL 和 JWT_SECRET
go run ./cmd/server         # 监听 :8080
```

启动时会自动 AutoMigrate 建表并初始化默认配置（含默认会员层级）。验证：

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
VITE_API_BASE_URL=https://api.yourdomain.com npm run build

# 后端
cd backend
go build -o server ./cmd/server
```

## 路线图

- [x] **阶段 0** — 脚手架与基础设施
- [x] **阶段 1** — 八字命理核心
- [x] **阶段 2** — 周公解梦 / 万年历 / 生肖 / 配对
- [x] **阶段 3** — 称骨 / 抽签 / 梅花 / 姓名 / 占星 / 紫微 / 星座 / 塔罗
- [x] **阶段 4** — JWT 认证 / 配额 / 多供应商 AI / 多层会员 / Admin 后台
- [ ] **阶段 5** — SEO / 缓存 / 英文 i18n / 内容合规

## 免责声明

本站内容仅供娱乐与文化参考，不构成任何专业建议。
