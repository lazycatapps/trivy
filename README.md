# Trivy Web UI - 容器镜像安全扫描工具

基于 Aqua Security Trivy 的容器镜像安全扫描工具，提供简洁的 Web UI 界面，用于对容器镜像进行漏洞扫描、配置检查和 SBOM 生成。

## 功能特性

- 🔍 容器镜像漏洞扫描（基于 Trivy）
- 📊 实时扫描进度和日志展示（SSE）
- 📋 漏洞结果可视化（支持按严重等级筛选）
- 📥 多格式报告导出（JSON/HTML/SARIF/SBOM）
- 📚 扫描历史管理（支持分页、搜索、过滤）
- ⚙️ 扫描配置保存与管理
- 🔐 支持私有镜像仓库认证
- 🔒 OIDC 统一认证支持（可选，用户数据隔离）
- 🎯 任务队列管理（串行执行，队列状态可视）
- ⚡ 前后端分离架构，易于部署

## 技术栈

**后端:**
- Go 1.25+
- Gin Web Framework v1.11.0+
- Trivy (Client-Server 模式)
- Cobra (命令行参数解析)
- Viper (配置管理)

**前端:**
- React 19.1+
- Ant Design 5.27+
- JavaScript (非 TypeScript)

## 前置要求

### 部署 Trivy Server

本项目使用 **Client-Server 模式**，需要先部署 Trivy Server。

**Docker Compose 部署（推荐）:**
```yaml
version: '3.8'
services:
  trivy-server:
    image: aquasec/trivy:latest
    container_name: trivy-server
    command:
      - server
      - --listen=0.0.0.0:4954
    volumes:
      - trivy-cache:/root/.cache/trivy
    ports:
      - "4954:4954"
    restart: unless-stopped

volumes:
  trivy-cache:
```

启动服务:
```bash
docker-compose up -d
```

详细配置请参考 [Trivy Server 部署文档](docs/trivy-server-deployment.md)

## 快速开始

### 方式一：LPK 部署（推荐）

适用于 Lazycat Cloud 平台部署。

#### 1. 构建前端

```bash
make build-dist
# 或手动构建
sh build.sh
```

#### 2. 构建后端镜像

```bash
make build-backend
# 或手动构建
cd backend
docker build -t registry.lazycat.cloud/trivy/backend:latest .
```

#### 3. 推送后端镜像

```bash
make push
# 或手动推送
docker push registry.lazycat.cloud/trivy/backend:latest
```

#### 4. 构建 LPK 包

```bash
make build-lpk
# 或手动构建
lzc build
```

#### 5. 部署 LPK

将生成的 `.lpk` 文件上传到 Lazycat Cloud 平台进行部署。

**环境变量说明：**

后端环境变量：
- `TRIVY_TRIVY_SERVER`: Trivy Server 地址（必需），例如 `http://trivy-server:4954`
- `TRIVY_DEFAULT_REGISTRY`: 默认镜像仓库地址前缀
- `TRIVY_ALLOW_PASSWORD_SAVE`: 是否允许保存密码到配置文件，默认 `false`
- `TRIVY_MAX_WORKERS`: 最大并发扫描工作线程数，默认 `5`
- `TRIVY_SCAN_RETENTION_DAYS`: 扫描历史保留天数，默认 `90`

OIDC 认证环境变量（可选）：
- `TRIVY_OIDC_CLIENT_ID=${LAZYCAT_AUTH_OIDC_CLIENT_ID}`
- `TRIVY_OIDC_CLIENT_SECRET=${LAZYCAT_AUTH_OIDC_CLIENT_SECRET}`
- `TRIVY_OIDC_ISSUER=${LAZYCAT_AUTH_OIDC_ISSUER}`
- `TRIVY_OIDC_REDIRECT_URL=https://${LAZYCAT_APP_DOMAIN}/api/v1/auth/callback`

LPK 部署说明：
- 前端通过 `application.routes` 配置自动代理到后端
- 修改后端地址或端口时，只需更新 `lzc-manifest.yml` 中的 `routes` 配置
- 详见 [部署指南](docs/DEPLOYMENT.md)

### 方式二：本地开发运行

#### 1. 启动后端服务

```bash
# 使用 Makefile（推荐）
make dev-backend

# 或手动启动
cd backend
go mod download
export TRIVY_TRIVY_SERVER="http://localhost:4954"
export TRIVY_DEFAULT_REGISTRY="docker.io/"
go run cmd/server/main.go
```

后端服务默认运行在 `http://localhost:8080`

可以通过 `-p` 或 `--port` 参数指定端口：

```bash
go run cmd/server/main.go --port 9090
```

#### 2. 启动前端服务

```bash
# 使用 Makefile（推荐）
make dev-frontend

# 或手动启动
cd frontend
npm install
npm start
```

前端服务默认运行在 `http://localhost:3000`

**配置说明：**

后端支持通过环境变量或命令行参数配置。主要配置项：
- `--trivy-server`: Trivy Server 地址（必需）
- `--default-registry`: 默认镜像仓库地址前缀
- `--timeout`: 扫描任务超时时间（秒），默认 600
- `--config-dir`: 配置文件存储目录，默认 `./configs`
- `--reports-dir`: 扫描报告存储目录，默认 `./reports`
- `--allow-password-save`: 是否允许保存密码，默认 `false`

环境变量格式：`TRIVY_` + 参数名（横线替换为下划线），例如 `TRIVY_TRIVY_SERVER`

### 使用说明

1. 打开浏览器访问 `http://localhost:3000`
2. 填写镜像地址（例如：`docker.io/library/nginx:latest`）
3. 如果是私有仓库，填写对应的用户名和密码
4. 选择扫描选项：
   - 漏洞严重等级过滤（CRITICAL, HIGH, MEDIUM, LOW, UNKNOWN）
   - 是否忽略未修复的漏洞
   - 扫描器类型（vuln, misconfig, secret, license）
   - 检测优先级（precise / comprehensive）
   - 包类型过滤（os, library）
5. 选择输出格式（JSON / Table / SARIF / CycloneDX / SPDX）
6. 点击"开始扫描"按钮
7. 实时查看扫描日志和进度
8. 扫描完成后查看漏洞结果，支持：
   - 按严重等级筛选
   - 按包名称搜索
   - 查看漏洞详细信息（CVE ID、描述、CVSS 评分、修复建议等）
   - 下载多格式报告（JSON/HTML/SARIF/SBOM）

**配置保存（高级功能）：**

- 通过高级选项面板可以保存常用配置，方便下次使用
- 支持多配置管理，每个配置最大 4KB
- 每个用户最多保存 1000 个配置
- 启用 OIDC 认证后，配置自动隔离，每个用户独立管理
- 密码保存功能默认禁用（`--allow-password-save=false`），需要时可通过参数启用

## 项目结构

```
trivy-web-ui/
├── backend/                   # Go 后端
│   ├── cmd/
│   │   └── server/
│   │       └── main.go        # 应用入口
│   ├── internal/              # 内部包
│   │   ├── config/            # 配置管理（已废弃，使用 types.Config）
│   │   ├── models/            # 数据模型
│   │   ├── types/             # 类型定义
│   │   ├── repository/        # 数据访问层
│   │   ├── service/           # 业务逻辑层
│   │   ├── handler/           # HTTP 处理层
│   │   ├── middleware/        # 中间件
│   │   ├── router/            # 路由管理
│   │   └── pkg/               # 工具包
│   ├── go.mod
│   ├── Dockerfile
│   └── .gitignore
├── frontend/                  # React 前端
│   ├── src/
│   │   ├── App.js             # 主组件
│   │   └── App.css            # 样式
│   ├── package.json
│   └── .gitignore
├── dist/                      # 构建输出目录（由 build.sh 生成）
│   └── web/                   # 前端静态文件
├── docs/                      # 文档目录
│   ├── trivy-server-deployment.md
│   └── DEPLOYMENT.md
├── build.sh                   # 前端构建脚本
├── lzc-build.yml              # LPK 构建配置
├── lzc-manifest.yml           # LPK 应用清单
├── icon.png                   # 应用图标
├── Makefile                   # 构建命令
├── CLAUDE.md                  # Claude 参考文档
└── README.md                  # 项目说明
```

## API 接口

### 扫描相关

- **POST** `/api/v1/scan` - 创建扫描任务
- **GET** `/api/v1/scan/:id` - 查询扫描任务状态
- **GET** `/api/v1/scan` - 查询扫描任务列表（支持分页、过滤）
- **GET** `/api/v1/scan/:id/logs` - 获取扫描任务的实时日志流 (SSE)
- **DELETE** `/api/v1/scan/:id` - 删除扫描任务
- **DELETE** `/api/v1/scan/:id/cancel` - 取消队列中的扫描任务
- **POST** `/api/v1/scan/:id/rescan` - 使用相同参数重新扫描

### 报告相关

- **GET** `/api/v1/scan/:id/report/:format` - 下载指定格式的扫描报告
  - 支持格式: `json`, `html`, `sarif`, `cyclonedx`, `spdx`, `table`
- **GET** `/api/v1/scan/:id/report/archive` - 批量下载所有格式的报告（ZIP）

### 配置相关

- **GET** `/api/v1/config/:name` - 获取已保存的用户配置
- **POST** `/api/v1/config/:name` - 保存用户配置
- **DELETE** `/api/v1/config/:name` - 删除已保存的用户配置
- **GET** `/api/v1/configs` - 获取所有配置名称列表
- **GET** `/api/v1/config/last-used` - 获取最后使用的配置名称

### 队列状态

- **GET** `/api/v1/queue/status` - 查询任务队列状态

### 认证相关（OIDC）

- **GET** `/api/v1/auth/login` - 跳转到 OIDC 登录页
- **GET** `/api/v1/auth/callback` - OIDC 认证回调
- **POST** `/api/v1/auth/logout` - 注销当前用户会话
- **GET** `/api/v1/auth/userinfo` - 获取当前用户信息

### 健康检查

- **GET** `/api/v1/health` - 健康检查接口

详细 API 文档请参考 [CLAUDE.md](CLAUDE.md)

## Makefile 命令

```bash
make help           # 显示所有可用命令
make build          # 构建后端镜像和前端 dist
make build-backend  # 构建后端 Docker 镜像
make build-dist     # 构建前端到 dist 目录
make build-lpk      # 构建 LPK 包（需要 lzc-cli）
make build-local    # 本地编译后端二进制
make push           # 推送后端镜像到仓库
make dev-backend    # 启动后端开发服务
make dev-frontend   # 启动前端开发服务
make test           # 运行测试
make clean          # 清理构建输出
```

## 调试功能

### 前端调试工具

应用内置了完整的调试功能，方便在移动端和生产环境排障：

1. **浮动调试按钮**
   - 位于页面右下角的虫子图标
   - 显示当前调试日志数量徽章
   - 点击打开调试日志面板

2. **调试日志记录**
   - 自动记录所有关键操作（初始化、API 请求、错误等）
   - VSCode 风格深色主题显示
   - 支持清空日志和自动滚动

3. **版本信息显示**
   - 页面底部显示应用版本、Git commit、分支名和构建时间
   - 鼠标悬停查看完整 commit 哈希
   - 方便快速确认部署版本

### 构建版本信息

`build.sh` 脚本会自动注入以下构建信息：
- Git commit 短哈希（如 `502089b`）
- Git 分支名
- 构建时间（UTC）

这些信息会编译到前端代码中，并显示在页面底部。

## 开发计划

- [x] 基础 Web UI 界面
- [x] 镜像扫描功能（支持多种扫描选项）
- [x] 实时日志查看功能（SSE）
- [x] 漏洞结果展示和筛选
- [x] 多格式报告导出（JSON/HTML/SARIF/SBOM）
- [x] 扫描历史管理（列表、分页、搜索）
- [x] 任务队列管理（串行执行）
- [x] 配置保存与管理
- [x] OIDC 认证支持
- [x] 前端调试工具（浮动按钮 + 日志面板）
- [x] Git 版本信息显示（commit、分支、构建时间）
- [x] 后端分层架构重构
- [x] LPK 打包支持
- [ ] 扫描结果对比功能（同镜像不同版本）
- [ ] 存储统计和配额管理
- [ ] 后台定时清理任务
- [ ] Docker 本地镜像扫描支持

## 文档

- [部署指南](docs/DEPLOYMENT.md) - 详细的环境变量配置和部署说明
- [Trivy Server 部署](docs/trivy-server-deployment.md) - Trivy Server 配置和部署
- [Claude 开发文档](CLAUDE.md) - 项目技术架构和开发指南

## 许可证

MIT

## 贡献

欢迎提交 Issue 和 Pull Request！
