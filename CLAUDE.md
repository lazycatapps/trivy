# Trivy Web UI 项目说明

## 项目概述

Trivy Web UI 是一个基于 Aqua Security Trivy 的容器镜像安全扫描工具，提供简洁的 Web UI 界面，用于对容器镜像进行漏洞扫描、配置检查和 SBOM 生成。

### 关于 Trivy

Trivy 是 Aqua Security 开源的综合性安全扫描工具，支持：
- **容器镜像扫描**: 检测操作系统包和应用依赖中的已知漏洞（CVE）
- **文件系统扫描**: 扫描本地文件系统中的安全问题
- **Git 仓库扫描**: 直接扫描代码仓库
- **配置扫描**: 检测 IaC（Infrastructure as Code）配置错误
- **密钥扫描**: 发现代码中的敏感信息泄露
- **SBOM 生成**: 生成软件物料清单（Software Bill of Materials）

Trivy 支持 client-server 模式：
- **Server 模式**: 托管漏洞数据库，后台自动更新
- **Client 模式**: 连接到 Trivy Server 进行远程扫描，无需本地下载数据库

### 项目目标

本项目旨在为 Trivy 提供友好的 Web UI 界面，让用户可以：
1. 通过 Web 界面提交镜像扫描任务
2. 实时查看扫描进度和日志
3. **查看和浏览扫描结果**（漏洞列表、配置错误、密钥泄露、许可证、SBOM 等）
4. **下载和导出扫描报告**（支持多种格式：JSON、HTML、SARIF、SBOM 等）
5. **历史扫描记录管理**（查看、对比、删除历史扫描）
6. 保存和管理常用扫描配置（用户维度）
7. 通过 OIDC 实现多用户隔离和权限管理

### 核心价值

- **便捷性**: 无需命令行操作，通过 Web 界面完成所有扫描工作
- **可视化**: 友好的漏洞展示和统计图表，快速了解镜像安全状况
- **可追溯**: 保留完整的扫描历史，支持结果对比和趋势分析
- **可共享**: 多格式报告导出，方便与团队共享和集成到 CI/CD
- **多租户**: OIDC 认证实现用户隔离，适合团队协作使用

## 技术架构

### 后端
- **语言**: Go 1.25+
- **Web 框架**: Gin v1.11.0+
- **核心依赖**:
  - `github.com/google/uuid` - 任务 ID 生成
  - `github.com/gin-gonic/gin` - HTTP 服务框架
  - `github.com/coreos/go-oidc/v3` - OIDC 认证库
  - `golang.org/x/oauth2` - OAuth2 客户端库
  - `github.com/spf13/cobra` - 命令行参数解析
  - `github.com/spf13/viper` - 配置管理
- **核心工具**: Trivy (通过命令行调用 trivy image 命令)
  - **使用 Client-Server 模式**: 连接到远程 Trivy Server 进行扫描
  - 支持镜像仓库认证 (--username, --password)
  - 支持 TLS 证书验证控制
  - 支持多种输出格式 (JSON, Table, SARIF, CycloneDX, SPDX)
  - **漏洞库管理**: 由 Trivy Server 端统一管理
    - Web UI 不维护本地漏洞数据库
    - 所有漏洞库配置（更新策略、镜像源等）需在 Trivy Server 端设置
    - Web UI 仅需配置 Trivy Server 地址（`--trivy-server` 或 `TRIVY_TRIVY_SERVER`）
- **配置管理**: 使用标准库 `os` 包读取环境变量
- **日志系统**: 自定义 Logger 接口，基于标准库 `log` 包
  - 支持 INFO/ERROR/DEBUG 三个日志级别
  - 统一的日志格式：`[LEVEL] timestamp message`
  - 输出到 stdout (INFO/DEBUG) 和 stderr (ERROR)
- **中间件**:
  - **CORS 中间件**
    - 默认允许所有来源 (`Access-Control-Allow-Origin: *`)
    - 可以通过环境变量配置为特定域名
    - 支持的方法: GET, POST, PUT, DELETE, OPTIONS
    - 支持的头: Content-Type, Authorization
  - **OIDC 认证中间件**
    - 支持基于 OpenID Connect (OIDC) 的统一认证
    - 自动验证会话 cookie
    - 支持公共端点白名单（如健康检查、认证回调）
    - API 请求认证失败返回 401，浏览器请求自动跳转登录页
- **会话管理**:
  - 内存会话存储（SessionService）
  - 会话 TTL: 7 天
  - 自动清理过期会话（每 10 分钟）
  - 支持会话刷新和注销

### 前端
- **框架**: React 19.1+
- **UI 库**: Ant Design 5.27+
- **构建工具**: Create React App (react-scripts 5.0+)
- **语言**: JavaScript (非 TypeScript)
- **状态管理**: React Hooks (useState, useEffect, useRef)
  - 不使用额外的状态管理库 (Redux/MobX)
  - 组件级状态管理，适合小型应用
- **测试框架**:
  - `@testing-library/react` - React 组件测试
  - `@testing-library/jest-dom` - Jest DOM 断言
  - `@testing-library/user-event` - 用户交互模拟
- **HTTP 通信**:
  - 使用浏览器原生 `fetch` API
  - 支持 EventSource (SSE) 接收实时日志流
- **开发环境**:
  - 后端 API 地址配置：通过环境变量 `BACKEND_API_URL` 注入到 `window._env_.BACKEND_API_URL`
    - 开发环境默认：`http://localhost:8080`
    - 生产环境示例：`http://host.lzcapp`、`https://api.example.com`
  - 无需额外的代理配置 (依赖后端 CORS 支持)
- **浏览器兼容性**:
  - 生产环境: >0.2% 浏览器市场份额，排除已停止维护的浏览器
  - 开发环境: 最新版本的 Chrome/Firefox/Safari
  - 支持移动端浏览器
- **调试功能**:
  - 全局错误捕获（错误和未处理的 Promise rejection）
  - 浮动调试按钮（右下角虫子图标）
  - 调试日志面板（记录所有关键操作和错误）
  - 支持清空日志和自动滚动
  - VSCode 风格深色主题显示
- **版本信息**:
  - 显示应用版本号
  - 显示 Git commit 短哈希（生产环境）
  - 显示 Git 分支名（非 main 分支时）
  - 显示构建时间（ISO 8601 格式）
  - 鼠标悬停显示完整 commit 哈希

## 核心功能

### 1. 镜像扫描
- **基本功能**
  - 支持扫描公开和私有镜像仓库中的容器镜像
  - 支持用户名/密码认证（私有仓库）
  - 支持 TLS 证书验证开关
  - 连接到 Trivy Server 进行远程扫描
- **扫描选项**
  - **漏洞严重等级过滤** (`--severity`): 选择要显示的漏洞等级
  - **忽略未修复漏洞** (`--ignore-unfixed`): 只显示有修复方案的漏洞
  - **扫描器类型** (`--scanners`): 选择扫描类型
    - `vuln`: 漏洞扫描（默认）
    - `misconfig`: 配置错误扫描
    - `secret`: 密钥泄露扫描
    - `license`: 软件许可证扫描
    - 支持组合，如 `vuln,secret`
  - **检测优先级** (`--detection-priority`):
    - `precise`: 精确模式，减少误报（默认）
    - `comprehensive`: 全面模式，检测更多漏洞但可能增加误报
  - **包类型过滤** (`--pkg-types`):
    - `os`: 仅扫描操作系统包
    - `library`: 仅扫描应用依赖包
- **输出格式**
  - **JSON** (`--format json`): 结构化输出，便于程序处理和展示
  - **Table** (`--format table`): 表格形式，便于人类阅读
  - **SARIF** (`--format sarif`): 静态分析结果交换格式
  - **CycloneDX** (`--format cyclonedx`): SBOM 标准格式（软件物料清单）
  - **SPDX** (`--format spdx`): SBOM 标准格式（另一种）
  - 默认使用 JSON 格式，便于前端解析和展示
- **任务队列与串行执行**
  - **重要约束**: Trivy 只支持串行扫描，不支持并行
  - 任务创建后立即返回任务 ID，加入任务队列
  - 后台 Worker 按 FIFO 顺序依次执行扫描任务
  - 队列机制确保同时只有一个 Trivy 进程在运行
  - 任务状态包含排队信息（如"队列中，前面还有 3 个任务"）
- **日志管理**
  - 实时捕获 Trivy 的 stdout 和 stderr 输出
  - 自动脱敏：日志中的凭据信息自动替换为 `***`
  - 日志缓冲区：1000 条消息

### 2. 扫描结果展示与导出
- **结果解析**
  - 后端存储 Trivy 的 JSON 格式输出（作为主数据源）
  - 前端解析 JSON 并以友好的方式展示
  - 支持多种结果类型：漏洞列表、配置错误、密钥泄露、SBOM
- **漏洞展示（主要功能）**
  - **列表视图**: 以表格形式展示所有漏洞
    - 列字段: CVE ID、漏洞名称、严重等级、包名称、已安装版本、修复版本、描述
    - 支持按严重等级筛选（CRITICAL/HIGH/MEDIUM/LOW/UNKNOWN）
    - 支持按包名称搜索
    - 严重等级用不同颜色标识（红色/橙色/黄色/蓝色/灰色）
  - **统计信息**: 顶部显示漏洞总数和各等级数量
    - 示例: "总计 156 个漏洞：CRITICAL (12), HIGH (45), MEDIUM (67), LOW (32)"
  - **详细信息**: 点击漏洞可展开查看详细描述、参考链接、CVSS 评分等

- **多格式报告导出（核心功能）**
  - **支持的报告格式**：
    1. **JSON** - 完整的结构化数据，便于程序处理
    2. **HTML** - 可视化报告，可直接在浏览器打开查看
       - 使用 Trivy 内置 HTML 模板生成
       - 包含漏洞统计图表和详细列表
    3. **SARIF** - 静态分析结果交换格式
       - 兼容 GitHub Security、SonarQube 等平台
    4. **CycloneDX SBOM** - 软件物料清单（JSON/XML）
    5. **SPDX SBOM** - 另一种 SBOM 标准格式
    6. **Table (Text)** - 纯文本表格格式，便于查看和分享

  - **报告生成机制**：
    - **主报告（JSON）**: 扫描完成时自动生成并存储
    - **格式转换**: 使用 Trivy 的 `convert` 子命令从 JSON 转换为其他格式
      - 后端动态生成：用户请求时实时转换
      - 临时文件管理：转换结果临时存储，定期清理（保留 24 小时）

  - **下载功能**：
    - **单文件下载**: 通过 API 端点下载指定格式的报告
      - `GET /api/v1/scan/:id/report/:format` - 返回文件流
      - 支持的 format: `json`, `html`, `sarif`, `cyclonedx`, `spdx`, `table`
      - 响应头设置正确的 Content-Type 和 Content-Disposition
      - 文件命名格式: `trivy-report-{image-name}-{timestamp}.{ext}`
    - **浏览器直接下载**: 前端通过 `<a download>` 或 `window.open()` 触发下载

  - **报告管理**：
    - **存储策略**:
      - JSON 主报告永久保存（或根据配置的保留期）
      - 转换后的报告临时存储 24 小时后自动清理
    - **并发控制**: 限制同时转换的报告数量，避免资源耗尽
    - **缓存机制**: 相同格式的报告在有效期内直接返回缓存副本

### 3. 实时日志系统
- **技术实现**
  - 使用 SSE (Server-Sent Events) 推送实时日志
  - 前端使用 EventSource API 接收日志流
  - 后端使用 channel 机制广播日志到所有监听客户端
- **多客户端支持**
  - 支持多个客户端同时订阅同一任务的日志
  - 每个监听器独立的 channel，缓冲区大小 1000 条
  - 任务完成后自动关闭所有 SSE 连接
- **日志显示**
  - Modal 弹窗显示，黑底绿字终端风格
  - 自动滚动到最新日志
  - Alert 组件实时显示任务状态

### 4. 任务状态管理
- **状态流转**
  - `queued`: 任务已创建，在队列中等待执行
  - `running`: 正在执行扫描操作
  - `completed`: 扫描成功完成
  - `failed`: 扫描失败
- **队列信息**
  - 记录任务在队列中的位置
  - 前端显示"队列中，前面还有 X 个任务"
  - 预估等待时间（基于历史平均扫描时长）
- **状态查询**
  - 前端通过轮询（1 秒间隔）获取任务状态和队列信息
  - 任务完成（completed/failed）后停止轮询
  - 支持通过 API 查询任务详情（状态、日志、错误信息等）
- **任务信息**
  - 记录创建时间、开始时间、结束时间
  - 保存完整的输出日志
  - 失败时记录错误信息

### 5. 扫描配置管理
- **默认值配置**
  - 支持通过环境变量配置 Trivy Server 地址
  - 支持通过环境变量配置默认镜像仓库地址前缀
  - 页面加载时自动填充默认值
- **用户配置保存（用户维度）**
  - 通过"高级选项"折叠面板提供"保存配置"功能
  - **保存内容**：
    - 镜像仓库地址前缀
    - 镜像仓库用户名和密码（base64 编码，可选）
    - TLS 验证选项
    - **扫描参数**：
      - 漏洞严重等级过滤（severity）
      - 是否忽略未修复漏洞（ignore-unfixed）
      - 扫描器类型（scanners）
      - 检测优先级（detection-priority）
      - 包类型过滤（pkg-types）
    - **输出参数**：
      - 输出格式（format: json/table/sarif/cyclonedx/spdx）
  - **密码存储安全控制**：
    - 通过 `--allow-password-save` 参数或 `TRIVY_ALLOW_PASSWORD_SAVE` 环境变量控制是否允许保存密码
    - **默认值：`false`（禁止保存密码，极致安全）**
  - **用户配置隔离**：
    - 启用 OIDC 认证时，每个用户的配置存储在独立目录：`/configs/users/{userID}/`
    - 未启用 OIDC 时，所有配置存储在共享目录：`/configs/`
    - 用户只能访问和管理自己的配置，完全隔离互不影响
  - **配置限制**：
    - **单个配置大小限制**：默认 4KB（4096 字节），可通过 `--max-config-size` 参数或 `TRIVY_MAX_CONFIG_SIZE` 环境变量配置
    - **配置数量限制**：默认每个用户最多 1000 个配置，可通过 `--max-config-files` 参数或 `TRIVY_MAX_CONFIG_FILES` 环境变量配置
    - 防止存储空间滥用和 DoS 攻击
  - 支持多配置管理：可保存多份配置并快速切换
  - 配置存储路径：基础路径可通过 `--config-dir` 参数或 `TRIVY_CONFIG_DIR` 环境变量指定，默认 `./configs/`（当前工作目录下的 configs 子目录）
  - 配置文件命名格式：`scan_config_NAME.json`
  - 记录最后使用的配置：`last_used.txt`

### 6. 扫描历史管理
- **历史记录列表**
  - **分页和过滤**：
  - **排序**：

- **历史记录详情**

- **批量操作**

- **扫描结果对比（高级功能）**

- **数据保留策略**
  - **保留期限**: 默认保留 90 天的扫描历史
    - 可通过 `--scan-retention-days` 参数或 `TRIVY_SCAN_RETENTION_DAYS` 环境变量配置
    - 设置为 0 表示永久保留（需谨慎使用）
  - **自动清理**: 后台定时任务（每天凌晨）清理过期记录
  - **存储优化**:
    - 压缩旧的 JSON 报告（gzip）节省空间
    - 删除超过保留期的临时转换报告
  - **存储限制**: 每个用户的扫描历史存储上限（默认 10GB）
    - 可通过 `--max-scan-storage` 参数配置
    - 达到上限时自动删除最旧的记录

- **导出和备份**
  - **导出历史列表**: 导出为 CSV/Excel 格式
    - 包含镜像名称、扫描时间、漏洞统计等元数据
    - 便于统计分析和报告
  - **批量备份**: 打包下载所有扫描报告（ZIP）
    - 包含所有历史扫描的 JSON 报告
    - 便于离线分析和归档

### 7. Web UI 设计
- **主页面布局**
  - **顶部导航栏**
    - 应用标题：Trivy Web UI
    - 用户信息（启用 OIDC 时显示）
    - 注销按钮（启用 OIDC 时显示）
  - **扫描表单区域**（卡片样式）
  - **扫描历史区域**（表格，Ant Design Table）

- **扫描结果页面**（独立页面）
  - **页面头部**
    - 返回按钮（返回主页）
    - 镜像名称和 tag（大标题）
    - 扫描时间和耗时信息
    - **操作按钮组**：
      - 重新扫描
      - 下载报告（下拉菜单，多格式选择）
      - 对比其他扫描（打开对比选择器）
      - 分享链接（复制当前页面 URL）

  - **顶部统计卡片区**（Ant Design Statistic）
    - **总漏洞数卡片**（居中，大字体）
    - **CRITICAL 卡片**（红色边框，显示数量）
    - **HIGH 卡片**（橙色边框）
    - **MEDIUM 卡片**（黄色边框）
    - **LOW 卡片**（蓝色边框）
    - 每个卡片支持点击快速筛选对应等级的漏洞

  - **Tab 标签页**（切换不同类型的扫描结果）

  - **报告下载浮动按钮**（右下角）
    - 点击展开下载格式菜单
    - 格式选项: JSON、HTML、SARIF、CycloneDX、SPDX、Table、全部打包
    - 显示文件大小预估
    - 一键下载选中格式
- **实时日志 Modal**
  - 黑底绿字终端风格
  - 实时显示扫描日志
  - 顶部显示任务状态 Alert（运行中/已完成/失败）
  - 自动滚动到最新日志
  - 关闭按钮
- **隐私保护**
  - 密码输入框使用 `type="password"` 属性
  - 不通过 URL 参数传递任何敏感信息
  - 凭据仅通过 POST 请求体发送到后端
  - 后端默认不存储凭据，仅在执行 Trivy 时使用
  - 日志中自动脱敏凭据信息
- **页面底部**（显示两行信息）
  - **第一行**: Web UI 版本信息
    - Trivy Web UI · v{APP_VERSION} · {GIT_COMMIT} · Copyright © {year} Lazycat Apps · GitHub
  - **第二行**: Trivy Server 版本信息（动态加载）
    - Trivy Server: {version} · Vuln DB: v{version} (更新: {updatedAt}) · Java DB: v{version} (更新: {updatedAt})
    - 信息来源: `GET /api/v1/trivy/version`
    - 页面加载时自动异步获取
    - 时间格式: `YYYY-MM-DD HH:mm:ss` (精确到秒)
    - 如果获取失败，不显示第二行

### 7. OIDC 认证（可选）
- **认证机制**
  - 支持基于 OpenID Connect (OIDC) 的统一认证
  - 与 Lazycat Cloud (heiyu.space) 微服认证系统集成
  - 自动获取用户 ID、邮箱、权限组信息
  - 支持管理员权限组识别（`ADMIN` 组）
- **认证流程**
  - 用户访问受保护资源时自动跳转到 OIDC 登录页
  - OIDC Provider 完成认证后回调到 `/api/v1/auth/callback`
  - 验证 ID Token 并提取用户信息（sub, email, groups）
  - 创建会话并设置 HttpOnly Cookie（7 天有效期）
  - 后续请求自动携带会话 Cookie 进行认证
- **配置方式**
  - 在 `lzc-manifest.yml` 中配置 `oidc_redirect_path: /api/v1/auth/callback`
  - 在 `services.backend.environment` 中引用 Lazycat 系统注入的环境变量：
    - `TRIVY_OIDC_CLIENT_ID=${LAZYCAT_AUTH_OIDC_CLIENT_ID}`: 客户端 ID
    - `TRIVY_OIDC_CLIENT_SECRET=${LAZYCAT_AUTH_OIDC_CLIENT_SECRET}`: 客户端密钥
    - `TRIVY_OIDC_ISSUER=${LAZYCAT_AUTH_OIDC_ISSUER}`: Issuer URL
    - `TRIVY_OIDC_REDIRECT_URL=https://${LAZYCAT_APP_DOMAIN}/api/v1/auth/callback`: 回调 URL
  - 当这些环境变量都配置后，OIDC 认证自动启用
  - **排查方法**：启动日志会输出 OIDC 配置状态，如果环境变量未注入，会显示 `(empty)`
- **认证端点**
  - `GET /api/v1/auth/login`: 跳转到 OIDC 登录页
  - `GET /api/v1/auth/callback`: OIDC 认证回调处理
  - `POST /api/v1/auth/logout`: 注销当前用户会话
  - `GET /api/v1/auth/userinfo`: 获取当前用户信息
- **会话管理**
  - 会话存储：内存存储（SessionService）
  - 会话有效期：7 天
  - 自动清理：每 10 分钟清理过期会话
  - Cookie 安全：HttpOnly, Secure (生产环境)
- **访问控制**
  - 公共端点：健康检查、认证相关端点
  - 受保护端点：所有镜像同步相关 API
  - 未认证访问：API 返回 401，浏览器跳转登录
- **关闭认证**
  - 如果不配置 OIDC 环境变量，认证功能自动禁用
  - 禁用后所有 API 端点均可直接访问

## 技术细节

### 1. 日志管理

- **日志库**: 自定义 Logger 接口 (`internal/pkg/logger`)
- **日志级别**:
- **日志格式**: `[LEVEL] timestamp message`
- **日志输出**:
- **日志轮转**: 当前不支持文件日志和日志轮转

### 2. 错误处理机制

- **统一错误类型**: `internal/pkg/errors/AppError`
- **预定义错误**:
  - `ErrTaskNotFound`: 任务不存在 (404)
  - `ErrInvalidInput`: 无效的输入参数 (400)
  - `ErrInternal`: 内部服务器错误 (500)
  - `ErrCommandFailed`: 命令执行失败 (500)
- **错误处理流程**:
  - Service 层抛出标准 Go error
  - Handler 层捕获并转换为统一的 JSON 错误响应
  - 返回格式: `{"error": "error message"}`
- **使用状态**: 已实现
  - errors 包已完成并可用
  - Handler 层使用标准错误响应格式

### 2.1. 输入验证与安全

- **输入验证库**: `internal/pkg/validator`
  - 防止命令注入、参数注入等安全漏洞
  - 所有用户输入在 Handler 层进行验证
- **验证规则**:
- **验证时机**:
  - `SyncHandler.SyncImage()`: 验证源镜像、目标镜像、架构、源凭据、目标凭据
  - `ImageHandler.InspectImage()`: 验证镜像名称和凭据
- **错误响应**:
  - 验证失败返回 400 Bad Request
  - 错误消息格式: `{"error": "validation error for field 'xxx': xxx"}`
- **安全原则**:
  - **纵深防御**: 虽然使用 `exec.Command()` 已避免 shell 注入，仍进行输入验证
  - **白名单优于黑名单**: 使用正则表达式定义允许的格式
  - **最小权限**: 只允许必要的字符集
  - **DoS 防护**: 限制输入长度，防止内存耗尽攻击

### 3. 命令执行与超时控制

- **命令执行方式**:
- **Trivy 命令示例**:
  ```bash
  trivy image \
    --server http://trivy-server:4954 \
    --severity HIGH,CRITICAL \
    --ignore-unfixed \
    --format json \
    --username myuser \
    --password mypass \
    docker.io/library/nginx:latest
  ```
- **超时控制**:
- **进程管理**:
  - 任务在 goroutine 中异步执行
  - 进程输出通过 pipe 实时读取
  - 任务完成、失败或超时后自动清理资源

### 4. 任务队列与串行执行

- **技术约束**:
  - **Trivy 只支持串行扫描**，不支持并行执行

- **任务队列设计**:
  - **队列类型**: FIFO（先进先出）队列
  - **队列实现**: 使用 Go channel 或内存队列（初期），可扩展为 Redis 队列（高可用场景）
  - **单 Worker 模式**: 后台运行一个 Worker goroutine，循环从队列中取任务执行

- **队列监控**:
  - 实时统计队列长度（待处理任务数）
  - 记录每个任务的等待时间和执行时间

- **用户体验优化**:
  - 任务状态返回时包含队列信息："队列中，前面还有 3 个任务"

- **资源管理**:
  - 每次只有一个 trivy 进程在运行，资源消耗可控

### 5. 数据存储

- **任务存储**: 内存存储 (`internal/repository/task_repository.go`)
  - 使用 `sync.RWMutex` 保证并发安全
  - 进程重启后任务历史丢失
  - 扩展性: 接口化设计,可替换为持久化存储 (数据库/文件)
- **日志缓冲**:
  - 每个任务维护独立的日志 slice
  - 每个 SSE 监听器独立的 channel,缓冲 1000 条消息
  - 任务完成后保留完整日志在内存中

### 6. 配置管理

- **命令行参数**:
  - `--host`: 指定服务监听地址，默认值 `0.0.0.0`
  - `--port`: 指定服务监听端口，默认值 8080
  - `--timeout`: 扫描任务超时时间（秒），默认 600（10分钟）
  - `--trivy-server`: **Trivy Server 地址**（必需），如 `http://trivy-server:4954`
  - `--default-registry`: 默认镜像仓库地址前缀
  - `--config-dir`: 配置文件存储目录，默认 `/configs`
  - `--reports-dir`: 扫描报告存储目录，默认 `/lzcapp/reports`
  - `--allow-password-save`: 是否允许在配置文件中保存密码，默认 `false`（极致安全）
  - `--max-config-size`: 单个配置文件最大大小（字节），默认 `4096`
  - `--max-config-files`: 每个用户最大配置文件数量，默认 `1000`
  - `--max-workers`: 最大并发扫描工作线程数，默认 `5`
  - `--scan-retention-days`: 扫描历史保留天数，默认 `90`（0 表示永久保留）
  - `--enable-docker-scan`: 是否启用 Docker 本地镜像扫描，默认 `false`（需挂载 Docker socket）
  - 使用 `github.com/spf13/cobra` 实现命令行参数解析
  - 使用 `github.com/spf13/viper` 读取环境变量
- **支持的环境变量**:
  - `TRIVY_` 开头的变量，命令行参数中的 `-` 替换为 `_`
      - 比如 `TRIVY_TRIVY_SERVER` 对应 `--trivy-server`
      - 比如 `TRIVY_DEFAULT_REGISTRY` 对应 `--default-registry`
      - 比如 `TRIVY_ALLOW_PASSWORD_SAVE` 对应 `--allow-password-save`
      - 比如 `TRIVY_MAX_CONFIG_SIZE` 对应 `--max-config-size`
      - 比如 `TRIVY_MAX_CONFIG_FILES` 对应 `--max-config-files`
      - 比如 `TRIVY_TIMEOUT` 对应 `--timeout`
      - 比如 `TRIVY_MAX_WORKERS` 对应 `--max-workers`
      - 比如 `TRIVY_SCAN_RETENTION_DAYS` 对应 `--scan-retention-days`
      - 比如 `TRIVY_ENABLE_DOCKER_SCAN` 对应 `--enable-docker-scan`

### 7. 安全性

- **输入验证** (详见 2.1 节):
- **凭据处理**:
- **命令注入防护**:
- **CORS 配置**:

### 8. 任务队列实现细节

- **队列数据结构**:

- **队列监控**:

- **优雅停机**:

- **任务取消**:

### 9. 前端技术细节

- **状态轮询**:
- **队列状态显示**:
- **实时日志**:
- **表单状态管理**:

## 架构设计

### 分层架构说明

后端采用标准的分层架构，职责清晰，易于维护和扩展：

1. **cmd/server** - 应用入口层

2. **handler** - HTTP 处理层

3. **service** - 业务逻辑层

4. **repository** - 数据访问层

5. **models** - 数据模型层

6. **middleware** - 中间件层

7. **pkg** - 工具包层

8. **config** - 配置管理层

### 设计优势

- ✅ **职责分离**: 每层职责明确，易于理解和维护
- ✅ **依赖注入**: 通过接口实现松耦合，易于测试
- ✅ **可扩展性**: 可轻松替换实现（如内存存储换成数据库）
- ✅ **可测试性**: 各层独立，便于单元测试
- ✅ **代码复用**: 公共逻辑提取到 service 和 pkg 层


## 设计反思与评估

### 需求扩展合理性分析

基于 Trivy 的完整能力，本次设计补充了以下核心功能：

#### 1. 多格式报告导出 ⭐⭐⭐⭐⭐ (5/5)
**合理性**: 充分利用 Trivy 原生支持的多种输出格式（JSON/HTML/SARIF/SBOM），通过 `trivy convert` 子命令实现格式转换，技术可行性高。

**优点**:
- 满足不同场景需求：开发者需要 JSON、安全团队需要 SARIF、合规需要 SBOM
- 动态转换 + 缓存机制兼顾性能和灵活性
- Content-Type 映射准确，符合 HTTP 规范

**优化建议**:
- HTML 报告考虑使用自定义模板，提供交互式图表
- ZIP 打包使用流式压缩，避免内存占用过高
- 添加并发转换控制（信号量限制最多 5 个并发）

#### 2. 扫描历史管理 ⭐⭐⭐⭐⭐ (5/5)
**合理性**: 保留历史扫描是实用功能，便于追踪镜像安全趋势。

**优点**:
- 数据保留策略（90天+存储限制 10GB）平衡了实用性和资源消耗
- 批量操作和过滤功能提升用户体验
- 支持 CSV 导出便于数据分析

**优化建议**:
- **扫描结果对比功能较复杂**，建议作为 v1.2 功能，MVP 先跳过
- 压缩旧报告（gzip）节省存储空间
- 增加存储统计页面，显示用户当前存储使用情况

#### 3. 增强的 Web UI 设计 ⭐⭐⭐⭐ (4/5)
**合理性**: Tab 标签页设计清晰展示不同类型的扫描结果，下载浮动按钮方便快速导出。

**优化建议**:
- **Tab 数量可能过多**（漏洞/配置错误/密钥/许可证/SBOM），建议仅在有数据时才显示对应 Tab
- 使用 Ant Design Table 的虚拟滚动优化大数据渲染性能
- 默认分页显示（每页 20 条），避免渲染卡顿

### 技术风险与优化方案

#### 风险 1: 报告存储空间快速增长
- **问题**: 一个镜像的 JSON 报告可能达到几 MB，频繁扫描会快速占用存储
- **方案**:
  - ✅ 压缩旧报告（gzip，压缩率约 80%）
  - ✅ 实现存储配额告警（达到 80% 时提示用户）
  - 🆕 增加存储统计和清理建议

#### 风险 2: Trivy Convert 性能瓶颈
- **问题**: HTML 转换可能需要几秒钟，多用户并发请求可能导致资源耗尽
- **方案**:
  - ✅ 使用信号量限制并发转换数量（最多 5 个）
  - ✅ 添加转换超时控制（60 秒）
  - 🆕 考虑使用后台任务队列异步处理转换请求

#### 风险 3: 前端大数据渲染性能
- **问题**: 扫描结果可能包含数百个漏洞，渲染大表格可能卡顿
- **方案**:
  - ✅ 使用 Ant Design Table 的虚拟滚动
  - ✅ 默认分页显示
  - 🆕 增加"仅显示 HIGH 及以上"快捷筛选

### 设计总体评分

| 维度 | 评分 | 说明 |
|------|------|------|
| **功能完整性** | ⭐⭐⭐⭐⭐ | 覆盖扫描、展示、导出、历史管理全流程 |
| **技术可行性** | ⭐⭐⭐⭐ | 基于 Trivy 原生能力，风险可控 |
| **用户体验** | ⭐⭐⭐⭐⭐ | UI 设计友好，操作便捷 |
| **安全性** | ⭐⭐⭐⭐⭐ | OIDC 认证、凭据保护、用户隔离完善 |
| **可扩展性** | ⭐⭐⭐⭐ | 分层架构清晰，需注意性能扩展 |
| **实现复杂度** | ⭐⭐⭐ | 中等复杂度，报告转换是关键 |

**总体评分**: ⭐⭐⭐⭐ (4.3/5) - **设计优秀，可以开始实施**

## 实施路线图

### MVP (v1.0) - 核心功能

**目标**: 提供完整的镜像扫描和结果查看功能

**功能清单**:
1. ✅ 镜像扫描
   - 基本参数：severity、scanners、ignore-unfixed
   - 镜像仓库认证（username/password）
   - TLS 验证控制
2. ✅ 漏洞结果展示
   - 统计卡片（总数 + 各等级数量）
   - 漏洞列表表格（CVE ID、严重等级、包名称、版本、描述）
   - 支持按等级筛选和包名搜索
3. ✅ 报告下载
   - JSON 格式（直接下载）
   - HTML 格式（Trivy convert 转换）
4. ✅ 扫描历史列表
   - 基本列表显示（镜像名称、时间、状态、漏洞统计）
   - 状态筛选、镜像名称搜索
   - 分页显示
   - 删除单个任务
5. ✅ 配置管理
   - 保存扫描配置（不含密码）
   - 加载已保存配置
   - 删除配置
6. ✅ OIDC 认证
   - 用户登录/登出
   - 用户级数据隔离

**技术任务**:
- [ ] 后端实现
  - [ ] Trivy 命令封装 (scan_service.go)
  - [ ] JSON 报告解析和漏洞统计 (scan_service.go)
  - [ ] HTML 报告转换 (使用 trivy convert)
  - [ ] 任务存储（内存 Repository）
  - [ ] 报告文件存储（文件系统）
- [ ] 前端实现
  - [ ] 扫描表单组件 (ScanForm.js)
  - [ ] 扫描结果展示 (ScanResult.js)
  - [ ] 历史列表 (ScanHistory.js)
  - [ ] 报告下载按钮
- [ ] 测试
  - [ ] 单元测试（service/repository 层）
  - [ ] API 集成测试
  - [ ] 前端组件测试

**时间估算**: 2-3 周

### v1.1 - 增强功能

**目标**: 支持更多报告格式和批量操作

**功能清单**:
1. SARIF/CycloneDX/SPDX 报告格式
2. ZIP 打包下载（多格式打包）
3. 批量删除历史记录
4. 密钥泄露扫描结果展示
5. 配置错误扫描结果展示
6. 历史列表高级过滤（时间范围选择器）
7. 重新扫描功能（复用参数）

**技术任务**:
- [ ] 后端实现
  - [ ] 报告格式转换服务 (多格式 convert)
  - [ ] ZIP 打包服务（流式压缩）
  - [ ] 批量删除 API
  - [ ] 重新扫描 API
- [ ] 前端实现
  - [ ] 格式选择下拉菜单
  - [ ] 批量操作工具栏
  - [ ] 密钥泄露/配置错误 Tab
  - [ ] 时间范围选择器组件

**时间估算**: 1-2 周

### v1.2 - 高级功能

**目标**: 提供数据分析和对比功能

**功能清单**:
1. 扫描结果对比（同镜像不同版本）
2. CSV 历史列表导出
3. 存储统计和配额管理页面
4. 许可证扫描和风险分类
5. 报告缓存优化（24小时缓存）
6. 后台定时清理任务

**技术任务**:
- [ ] 后端实现
  - [ ] 扫描结果对比算法
  - [ ] CSV 导出服务
  - [ ] 存储统计服务
  - [ ] 缓存管理和清理 cron
- [ ] 前端实现
  - [ ] 对比结果展示页面
  - [ ] 存储统计仪表盘
  - [ ] 许可证风险分类展示

**时间估算**: 2-3 周

### v2.0 - 企业功能（未来规划）

**功能清单**:
1. 定期扫描和邮件通知
2. 自定义 HTML 报告模板
3. 扫描结果 REST API（供 CI/CD 集成）
4. 仪表盘和趋势分析图表
5. Webhook 集成（钉钉/Slack 通知）
6. 多 Trivy Server 负载均衡

**时间估算**: 4-6 周

### 性能指标

**响应时间**:
- 单次扫描完成时间: < 30 秒（取决于镜像大小和漏洞数量）
- 报告转换时间: < 10 秒
- 历史列表加载时间: < 2 秒
- 报告下载响应时间: < 1 秒（JSON 直接返回）

**队列与并发**:
- **扫描并发**: 串行执行，同时只能运行 1 个扫描任务（Trivy 技术约束）
- **队列容量**: 无限制（但队列过长时建议前端提示用户）
- **报告转换并发**: 5 个（信号量控制）
- **并发用户数**: 100+（OIDC 会话管理）
- **队列等待时间**: 取决于队列长度和平均扫描时长
  - 示例：队列中有 5 个任务，平均扫描 30 秒，预计等待 2.5 分钟

**吞吐量估算**:
- 平均扫描时长: 30 秒
- 理论最大吞吐量: 120 个任务/小时
- 实际吞吐量: 约 100 个任务/小时（考虑失败重试和队列调度开销）

**存储限制**:
- 单用户存储上限: 10GB（可配置）
- 单个报告大小限制: 100MB
- 报告保留期: 90 天（可配置）

## 开发状态

### 已完成
- ✅ 项目目录结构（2025-10-03）
- ✅ Go 后端基础框架
- ✅ React 前端基础 UI
- ✅ 镜像扫描功能（基于 Trivy）
- ✅ 任务状态管理和跟踪
- ✅ 任务队列管理（串行执行）
- ✅ 实时日志输出（SSE - Server-Sent Events）
- ✅ 前端任务日志展示
- ✅ 漏洞结果展示和筛选
- ✅ 多格式报告导出
- ✅ 扫描历史管理
- ✅ 配置保存与管理
- ✅ OIDC 认证功能
- ✅ 后端分层架构重构（2025-10-01）
- ✅ 单元测试覆盖
- ✅ 路由管理独立（router 层）
- ✅ Dockerfile 优化
- ✅ LPK 打包支持

### 待实现
- [ ] 扫描结果对比功能（同镜像不同版本）
- [ ] 存储统计和配额管理页面
- [ ] 后台定时清理任务（清理过期扫描历史和临时报告）
- [ ] Docker 本地镜像扫描支持（需挂载 Docker socket）
- [ ] CSV/Excel 历史列表导出
- [ ] 定期扫描和邮件通知
- [ ] Webhook 集成（钉钉/Slack 通知）

## Trivy Server 部署配置

本项目使用 **Client-Server 模式**，Web UI 作为 Trivy Client 连接到远程 Trivy Server 进行扫描。

### Trivy Server 职责

- 托管和管理漏洞数据库（DB 和 Java DB）
- 自动更新漏洞库（可配置更新策略和镜像源）
- 提供扫描服务 API 给多个 Client
- 资源共享：多个 Web UI 实例共享同一个漏洞库

### 推荐部署方案

**详细配置请参考**: `docs/trivy-server-deployment.md`

#### 基础部署（Docker Compose）

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

#### 高级配置选项

```yaml
command:
  - server
  - --listen=0.0.0.0:4954
  # 漏洞库自动更新（推荐）
  - --skip-db-update=false
  # 使用国内镜像源加速（可选）
  # - --db-repository=https://trivy-db.example.com/
  # - --java-db-repository=https://trivy-java-db.example.com/
```

### Web UI 配置

只需配置 Trivy Server 地址：

```yaml
# lzc-manifest.yml
environment:
  - TRIVY_TRIVY_SERVER=http://trivy-server:4954
```

### 常见问题

#### Q: 漏洞库如何更新？

**A**: 由 Trivy Server 自动管理，Web UI 无需配置。建议 Server 端设置 `--skip-db-update=false` 启用自动更新。

#### Q: 能否自定义漏洞库源？

**A**: 可以，在 Trivy Server 启动命令中配置 `--db-repository` 和 `--java-db-repository` 参数。

#### Q: 离线环境如何使用？

**A**: 在 Trivy Server 容器构建时预下载漏洞库，启动时设置 `--skip-db-update=true`。详见 `docs/trivy-server-deployment.md`。

## 开发指南

### 启动开发环境

1. 后端:
```bash
cd backend
# 可以通过环境变量指定默认镜像地址
export DEFAULT_SOURCE_REGISTRY="registry.lazycat.cloud/"
export DEFAULT_DEST_REGISTRY="registry.mybox.heiyu.space/"
go run cmd/server/main.go         # 默认端口 8080
go run cmd/server/main.go -port 9090  # 指定端口

# 或使用 Makefile
make dev-backend
make build-local  # 编译二进制文件
```

2. 前端:
```bash
cd frontend
npm start
```

### 环境变量使用示例

打开 Web UI 后，源镜像地址和目标镜像地址输入框会自动填充为环境变量指定的默认值。
如果未设置环境变量，则输入框为空。

### 注意事项
- 确保系统已安装 Skopeo
- 后端需要有执行 Skopeo 命令的权限
- CORS 已在后端配置，允许跨域请求
- 前端默认请求后端地址: http://localhost:8080
- 架构查询按钮和下拉框使用 flexbox 布局保持在同一行
- 查询结果显示在按钮下方，使用 Ant Design 的 Tag 组件

### 构建和部署

#### 构建信息注入
`build.sh` 脚本会自动注入 Git 版本信息：
- `REACT_APP_GIT_COMMIT`: Git commit 短哈希（7位）
- `REACT_APP_GIT_COMMIT_FULL`: Git commit 完整哈希
- `REACT_APP_GIT_BRANCH`: Git 分支名
- `REACT_APP_BUILD_TIME`: 构建时间（UTC，ISO 8601 格式）

这些信息会显示在页面底部，方便排障和版本追踪。

#### 构建命令
```bash
# 完整构建（清理 + 重新构建）
./build.sh

# 快速构建（如果 dist 存在则跳过）
./build.sh fast

# 使用 Makefile
make build-dist      # 构建前端到 dist 目录
make build           # 构建后端镜像和前端
make deploy          # 完整部署流程
make deploy-fast     # 快速部署（不重新构建前端/后端）
```

## 后续优化方向

1. 添加任务队列，支持批量同步
2. 支持进度条显示
3. 添加 WebSocket 实现实时状态更新
4. 支持镜像列表批量导入（YAML/JSON）
5. 添加用户认证和权限管理
6. 支持 Docker Compose 一键部署
