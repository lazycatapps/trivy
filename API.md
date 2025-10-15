## API 设计

### POST /api/v1/scan
创建镜像扫描任务

**请求参数:**
```json
{
  "image": "docker.io/library/nginx:latest",
  "username": "user1",
  "password": "pass1",
  "tlsVerify": true,
  "severity": ["HIGH", "CRITICAL"],
  "ignoreUnfixed": false,
  "scanners": ["vuln", "secret"],
  "detectionPriority": "precise",
  "pkgTypes": ["os", "library"],
  "format": "json"
}
```

**字段说明:**
- `image` (必填): 镜像地址
- `username` (可选): 镜像仓库用户名
- `password` (可选): 镜像仓库密码
- `tlsVerify` (可选): 是否验证 TLS 证书,默认 `true`
- `severity` (可选): 漏洞严重等级数组,可选值: `UNKNOWN`, `LOW`, `MEDIUM`, `HIGH`, `CRITICAL`,默认全部
- `ignoreUnfixed` (可选): 是否忽略未修复的漏洞,默认 `false`
- `scanners` (可选): 扫描器类型数组,可选值: `vuln`, `misconfig`, `secret`, `license`,默认 `["vuln"]`
- `detectionPriority` (可选): 检测优先级,可选值: `precise`(精确), `comprehensive`(全面),默认 `precise`
- `pkgTypes` (可选): 包类型数组,可选值: `os`, `library`,默认全部
- `format` (可选): 输出格式,可选值: `json`, `table`, `sarif`, `cyclonedx`, `spdx`,默认 `json`

**成功响应 (200):**
```json
{
  "message": "Scan started",
  "id": "550e8400-e29b-41d4-a716-446655440000"
}
```

**错误响应:**
- **400 Bad Request** - 请求参数错误
  ```json
  {
    "error": "image is required"
  }
  ```
- **500 Internal Server Error** - 服务器内部错误
  ```json
  {
    "error": "Failed to create scan task"
  }
  ```

### GET /api/v1/scan/:id
查询扫描任务状态

**路径参数:**
- `id`: 任务 ID (UUID 格式)

**成功响应 (200):**
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "image": "docker.io/library/nginx:latest",
  "status": "completed",
  "message": "Scan completed successfully",
  "startTime": "2025-10-01T10:30:00Z",
  "endTime": "2025-10-01T10:32:15Z",
  "result": {
    "format": "json",
    "data": "{ ... }",
    "summary": {
      "total": 45,
      "critical": 3,
      "high": 12,
      "medium": 20,
      "low": 10,
      "unknown": 0
    }
  },
  "output": "Task started...\nScan completed...",
  "errorOutput": ""
}
```

**字段说明:**
- `status`: 任务状态,可选值: `queued`、`running`、`completed`、`failed`
- `message`: 状态描述信息
- `queuePosition` (可选): 队列中的位置 (仅 `status=queued` 时有值)
- `estimatedWaitTime` (可选): 预估等待时间（秒）(仅 `status=queued` 时有值)
- `result`: 扫描结果对象 (仅 `status=completed` 时有值)
  - `format`: 输出格式 (json/table/sarif/cyclonedx/spdx)
  - `data`: Trivy 原始输出结果 (JSON 字符串或 plain text)
  - `summary`: 漏洞统计摘要 (仅 format=json 时解析生成)
- `output`: 完整的日志输出
- `errorOutput`: 错误信息 (仅 `status=failed` 时有值)

**错误响应:**
- **404 Not Found** - 任务不存在
  ```json
  {
    "error": "Task not found"
  }
  ```
- **500 Internal Server Error** - 服务器内部错误
  ```json
  {
    "error": "Failed to get task"
  }
  ```

### GET /api/v1/scan
查询扫描任务列表

**查询参数:**
- `page` (可选): 页码,从 1 开始,默认 1
- `pageSize` (可选): 每页数量,默认 20,最大 100
- `status` (可选): 过滤任务状态,可选值: `pending`、`running`、`completed`、`failed`
- `sortBy` (可选): 排序字段,可选值: `startTime`、`endTime`,默认 `startTime`
- `sortOrder` (可选): 排序方向,可选值: `asc`、`desc`,默认 `desc`

**成功响应 (200):**
```json
{
  "total": 100,
  "page": 1,
  "pageSize": 20,
  "tasks": [
    {
      "id": "550e8400-e29b-41d4-a716-446655440000",
      "image": "docker.io/library/nginx:latest",
      "status": "completed",
      "message": "Scan completed successfully",
      "startTime": "2025-10-01T10:30:00Z",
      "endTime": "2025-10-01T10:32:15Z",
      "summary": {
        "total": 45,
        "critical": 3,
        "high": 12,
        "medium": 20,
        "low": 10,
        "unknown": 0
      }
    }
  ]
}
```

### GET /api/v1/queue/status
查询任务队列状态

**成功响应 (200):**
```json
{
  "queueLength": 5,
  "runningTaskId": "550e8400-e29b-41d4-a716-446655440000",
  "averageWaitTime": 120,
  "averageScanTime": 45
}
```

**字段说明:**
- `queueLength`: 当前队列中等待的任务数量
- `runningTaskId`: 正在执行的任务 ID (如果有)
- `averageWaitTime`: 平均等待时间（秒），基于最近 20 个任务计算
- `averageScanTime`: 平均扫描时长（秒），基于最近 20 个已完成任务计算

### GET /api/v1/scan/:id/logs
获取扫描任务的实时日志流 (SSE)

**路径参数:**
- `id`: 任务 ID (UUID 格式)

**响应头:**
```
Content-Type: text/event-stream
Cache-Control: no-cache
Connection: keep-alive
```

**响应格式 (Server-Sent Events):**
```
data: Task started at 2025-10-01T10:30:00+08:00
data: Executing trivy image scan
data: Connecting to Trivy Server at http://trivy-server:4954
data: Scanning docker.io/library/nginx:latest
data: Analyzing OS packages...
data: Analyzing application dependencies...
data: Found 45 vulnerabilities
data: Scan completed at 2025-10-01T10:32:15+08:00
```

**错误响应:**
- **404 Not Found** - 任务不存在
  ```json
  {
    "error": "Task not found"
  }
  ```

### GET /api/v1/config/:name
获取已保存的用户配置

**路径参数:**
- `name`: 配置名称

**成功响应 (200):**
```json
{
  "registryPrefix": "registry.example.com/",
  "username": "user1",
  "password": "cGFzczE=",
  "tlsVerify": true,
  "severity": ["HIGH", "CRITICAL"],
  "ignoreUnfixed": false,
  "scanners": ["vuln", "secret"],
  "detectionPriority": "precise",
  "pkgTypes": ["os", "library"],
  "format": "json"
}
```

**说明:**
- 返回保存的配置，如无配置则返回空对象 `{}`
- `password` 为 base64 编码后的密码（如果服务器允许保存密码）
- 前端需要使用 `atob()` 解码密码

### POST /api/v1/config/:name
保存用户配置

**路径参数:**
- `name`: 配置名称（只能包含字母、数字、点、横线和下划线）

**请求参数:**
```json
{
  "registryPrefix": "registry.example.com/",
  "username": "user1",
  "password": "cGFzczE=",
  "tlsVerify": true,
  "severity": ["HIGH", "CRITICAL"],
  "ignoreUnfixed": false,
  "scanners": ["vuln", "secret"],
  "detectionPriority": "precise",
  "pkgTypes": ["os", "library"],
  "format": "json"
}
```

**字段说明:**
- 所有字段均为可选
- `password` 应为 base64 编码后的密码
- 前端需要使用 `btoa()` 编码密码
- 配置会覆盖之前保存的同名配置
- 是否保存密码取决于服务器配置（`--allow-password-save`）

**成功响应 (200):**
```json
{
  "message": "Configuration saved successfully"
}
```

**错误响应:**
- **400 Bad Request** - 请求参数错误
  ```json
  {
    "error": "Configuration size (5000 bytes) exceeds maximum allowed size (4096 bytes)"
  }
  ```
  ```json
  {
    "error": "Maximum number of configs (1000) reached"
  }
  ```
- **500 Internal Server Error** - 服务器内部错误

### DELETE /api/v1/config/:name
删除已保存的用户配置

**路径参数:**
- `name`: 配置名称

**成功响应 (200):**
```json
{
  "message": "Configuration deleted successfully"
}
```

**说明:**
- 删除后，配置文件会被移除
- 如果配置文件不存在，仍返回成功

### GET /api/v1/configs
获取所有配置名称列表

**成功响应 (200):**
```json
{
  "configs": ["default", "prod-env", "test-config"]
}
```

**说明:**
- 返回所有已保存的配置名称列表（按字母顺序排序）
- 如果没有配置，返回空数组

### GET /api/v1/config/last-used
获取最后使用的配置名称

**成功响应 (200):**
```json
{
  "name": "default"
}
```

**说明:**
- 返回最后使用的配置名称
- 如果没有记录，返回空字符串

### GET /api/v1/scan/:id/report/:format
下载指定格式的扫描报告

**路径参数:**
- `id`: 任务 ID (UUID 格式)
- `format`: 报告格式，可选值: `json`, `html`, `sarif`, `cyclonedx`, `spdx`, `table`

**查询参数:**
- `download` (可选): 是否作为附件下载，默认 `true`

**成功响应 (200):**
- 返回文件流
- 响应头:
  ```
  Content-Type: application/json (或其他对应的 MIME 类型)
  Content-Disposition: attachment; filename="trivy-report-nginx-20251002-120000.json"
  Content-Length: 12345
  ```

**说明:**
- **JSON 格式**: 直接返回存储的主报告 JSON
- **其他格式**: 后端使用 Trivy convert 子命令动态转换
  - 转换结果临时缓存 24 小时
  - 相同格式的重复请求直接返回缓存副本
- **文件命名规则**: `trivy-report-{image-name}-{timestamp}.{ext}`
  - image-name 中的特殊字符替换为下划线
  - timestamp 格式: `YYYYMMDD-HHmmss`
- **Content-Type 映射**:
  - `json` → `application/json`
  - `html` → `text/html`
  - `sarif` → `application/sarif+json`
  - `cyclonedx` → `application/vnd.cyclonedx+json`
  - `spdx` → `application/spdx+json`
  - `table` → `text/plain`

**错误响应:**
- **404 Not Found** - 任务不存在或任务未完成
  ```json
  {
    "error": "Task not found or not completed"
  }
  ```
- **400 Bad Request** - 不支持的格式
  ```json
  {
    "error": "Unsupported format: xyz"
  }
  ```
- **500 Internal Server Error** - 报告生成失败
  ```json
  {
    "error": "Failed to generate report: conversion error"
  }
  ```

### GET /api/v1/scan/:id/report/archive
批量下载所有格式的报告（ZIP 压缩包）

**路径参数:**
- `id`: 任务 ID (UUID 格式)

**查询参数:**
- `formats` (可选): 指定要包含的格式，逗号分隔，默认全部
  - 示例: `formats=json,html,sarif`

**成功响应 (200):**
- 返回 ZIP 文件流
- 响应头:
  ```
  Content-Type: application/zip
  Content-Disposition: attachment; filename="trivy-reports-nginx-20251002-120000.zip"
  Content-Length: 123456
  ```

**ZIP 包结构:**
```
trivy-reports-nginx-20251002-120000.zip
├── trivy-report-nginx-20251002-120000.json
├── trivy-report-nginx-20251002-120000.html
├── trivy-report-nginx-20251002-120000.sarif
├── trivy-report-nginx-20251002-120000.cyclonedx.json
├── trivy-report-nginx-20251002-120000.spdx.json
└── trivy-report-nginx-20251002-120000.txt (table format)
```

**说明:**
- 后端动态生成 ZIP 包
- 利用流式压缩，避免内存占用过高
- 生成的 ZIP 文件不缓存，每次请求实时生成

**错误响应:**
- **404 Not Found** - 任务不存在或任务未完成
- **500 Internal Server Error** - ZIP 打包失败

### DELETE /api/v1/scan/:id
删除扫描任务及其报告

**路径参数:**
- `id`: 任务 ID (UUID 格式)

**成功响应 (200):**
```json
{
  "message": "Scan task deleted successfully"
}
```

**说明:**
- 删除任务记录
- 删除关联的 JSON 报告文件
- 删除所有缓存的转换报告
- 删除日志记录

**错误响应:**
- **404 Not Found** - 任务不存在
- **403 Forbidden** - 无权限删除（OIDC 启用时，只能删除自己的任务）

### DELETE /api/v1/scan/:id/cancel
取消队列中的扫描任务

**路径参数:**
- `id`: 任务 ID (UUID 格式)

**成功响应 (200):**
```json
{
  "message": "Scan task cancelled successfully"
}
```

**说明:**
- 仅支持取消队列中的任务（`status=queued`）
- 不支持取消正在执行的任务（`status=running`），因为 Trivy 进程已启动
- 取消后任务状态变为 `failed`，错误信息："任务已被用户取消"

**错误响应:**
- **404 Not Found** - 任务不存在
- **400 Bad Request** - 任务不在队列中（已开始执行或已完成）
  ```json
  {
    "error": "Cannot cancel task: task is already running or completed"
  }
  ```
- **403 Forbidden** - 无权限取消（OIDC 启用时，只能取消自己的任务）

### POST /api/v1/scan/:id/rescan
使用相同参数重新扫描

**路径参数:**
- `id`: 原任务 ID (UUID 格式)

**成功响应 (200):**
```json
{
  "message": "Scan started",
  "id": "new-task-uuid"
}
```

**说明:**
- 复用原任务的所有扫描参数（镜像、severity、scanners 等）
- 创建新的扫描任务
- 不复用凭据（用户需重新输入密码，安全考虑）

**错误响应:**
- **404 Not Found** - 原任务不存在

### GET /api/v1/scan/export
导出扫描历史列表（CSV/Excel）

**查询参数:**
- `format`: 导出格式，可选值: `csv`, `excel`
- `startDate` (可选): 开始日期 (ISO 8601)
- `endDate` (可选): 结束日期 (ISO 8601)
- `status` (可选): 过滤任务状态

**成功响应 (200):**
- CSV 格式:
  ```
  Content-Type: text/csv
  Content-Disposition: attachment; filename="scan-history-20251002.csv"
  ```
- Excel 格式:
  ```
  Content-Type: application/vnd.openxmlformats-officedocument.spreadsheetml.sheet
  Content-Disposition: attachment; filename="scan-history-20251002.xlsx"
  ```

**导出字段:**
- 镜像名称
- 扫描时间
- 扫描状态
- 扫描耗时
- 漏洞总数
- CRITICAL 数量
- HIGH 数量
- MEDIUM 数量
- LOW 数量
- UNKNOWN 数量

**说明:**
- 仅导出当前用户的扫描历史（OIDC 启用时）
- 支持时间范围和状态过滤

### GET /api/v1/health
健康检查接口

**成功响应 (200):**
```json
{
  "status": "ok"
}
```

### GET /api/v1/trivy/version
获取 Trivy Server 版本信息

**说明:**
- 公共端点，无需认证
- 返回 Trivy Server 的版本号和漏洞库信息
- 用于页面底部展示和扫描任务记录

**成功响应 (200):**
```json
{
  "version": "0.54.1",
  "vulnerabilityDB": {
    "version": 2,
    "nextUpdate": "2024-08-12T12:11:34.28Z",
    "updatedAt": "2024-08-12T06:11:34.28Z",
    "downloadedAt": "2024-08-12T09:59:01.59Z"
  },
  "javaDB": {
    "version": 1,
    "nextUpdate": "2024-08-12T12:15:20.00Z",
    "updatedAt": "2024-08-12T06:15:20.00Z",
    "downloadedAt": "2024-08-12T10:00:00.00Z"
  }
}
```

**字段说明:**
- `version`: Trivy Server 版本号
- `vulnerabilityDB`: 通用漏洞数据库信息
  - `version`: 数据库版本号
  - `nextUpdate`: 下次更新时间
  - `updatedAt`: 最后更新时间
  - `downloadedAt`: 下载时间
- `javaDB`: Java 漏洞数据库信息（字段同上）

**错误响应:**
- **500 Internal Server Error** - 无法连接到 Trivy Server 或解析失败
  ```json
  {
    "error": "Failed to get Trivy Server version: connection refused"
  }
  ```

### GET /api/v1/auth/login
跳转到 OIDC Provider 进行认证登录

**说明:**
- 生成随机 state 用于 CSRF 防护
- 将 state 保存到 cookie（10 分钟有效期）
- 重定向到 OIDC Provider 的授权页面
- 仅在启用 OIDC 认证时可用

**响应:**
- **302 Found** - 重定向到 OIDC Provider 授权页面
- **503 Service Unavailable** - OIDC 认证未启用

### GET /api/v1/auth/callback
OIDC 认证回调处理

**查询参数:**
- `code` (必填): 授权码
- `state` (必填): CSRF 防护令牌

**处理流程:**
1. 验证 state 与 cookie 中的 state 是否匹配
2. 使用授权码交换访问令牌和 ID Token
3. 验证 ID Token 签名
4. 提取用户信息（sub, email, groups）
5. 创建会话并设置 session cookie
6. 重定向到首页

**响应:**
- **302 Found** - 认证成功，重定向到首页
- **400 Bad Request** - State 不匹配或缺少参数
- **500 Internal Server Error** - Token 验证失败或内部错误

### POST /api/v1/auth/logout
注销当前用户会话

**说明:**
- 删除服务器端会话
- 清除客户端 session cookie

**成功响应 (200):**
```json
{
  "message": "Logged out successfully"
}
```

### GET /api/v1/auth/userinfo
获取当前登录用户信息

**成功响应 (200) - 已认证:**
```json
{
  "authenticated": true,
  "user_id": "user-uuid",
  "email": "user@example.com",
  "groups": ["ADMIN", "USER"],
  "is_admin": true
}
```

**成功响应 (200) - 未认证:**
```json
{
  "authenticated": false
}
```

**说明:**
- 如果 OIDC 未启用，总是返回 `authenticated: false`
- `is_admin` 字段表示用户是否在 `ADMIN` 权限组中
