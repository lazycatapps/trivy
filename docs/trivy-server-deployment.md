# Trivy Server 部署指南

## 概述

Trivy Web UI 采用 **Client-Server 架构**，需要先部署 Trivy Server，然后配置 Web UI 连接到该 Server。

**架构优势**：
- ✅ **资源共享**：多个 Web UI 实例共享同一个漏洞数据库
- ✅ **集中管理**：统一管理漏洞库更新策略
- ✅ **性能优化**：Web UI 无需下载和维护本地数据库
- ✅ **扩展性强**：支持水平扩展 Web UI 实例

## 部署方案

### 方案 1: Docker Compose（推荐用于开发/测试）

#### 基础配置

创建 `docker-compose.yml`：

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
    healthcheck:
      test: ["CMD", "wget", "--quiet", "--tries=1", "--spider", "http://localhost:4954/healthz"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 10s

  trivy-web-ui:
    image: registry.lazycat.cloud/trivy-web:latest
    container_name: trivy-web-ui
    environment:
      - TRIVY_TRIVY_SERVER=http://trivy-server:4954
      - TRIVY_TIMEOUT=600
      - TRIVY_MAX_WORKERS=5
    ports:
      - "8080:8080"
    depends_on:
      - trivy-server
    restart: unless-stopped

volumes:
  trivy-cache:
    driver: local
```

#### 启动服务

```bash
docker-compose up -d
```

#### 查看日志

```bash
# Trivy Server 日志
docker-compose logs -f trivy-server

# Web UI 日志
docker-compose logs -f trivy-web-ui
```

### 方案 2: 公网环境部署（自动更新漏洞库）

适用于有公网访问的环境，推荐启用自动更新获取最新漏洞数据。

```yaml
version: '3.8'

services:
  trivy-server:
    image: aquasec/trivy:latest
    container_name: trivy-server
    command:
      - server
      - --listen=0.0.0.0:4954
      - --skip-db-update=false  # 启用自动更新
    volumes:
      - trivy-cache:/root/.cache/trivy
    ports:
      - "4954:4954"
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "wget", "--quiet", "--tries=1", "--spider", "http://localhost:4954/healthz"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 60s  # 首次启动需要下载数据库，延长启动时间

volumes:
  trivy-cache:
    driver: local
```

**说明**：
- 首次启动会自动下载漏洞数据库（约 100MB），可能需要几分钟
- 后续每 12 小时自动检查更新
- 确保服务器可以访问 `ghcr.io` (Trivy 官方镜像源)

### 方案 3: 使用国内镜像源加速

适用于国内网络环境，使用镜像源加速数据库下载。

```yaml
version: '3.8'

services:
  trivy-server:
    image: aquasec/trivy:latest
    container_name: trivy-server
    command:
      - server
      - --listen=0.0.0.0:4954
      - --skip-db-update=false
      # 配置国内镜像源（示例，请替换为实际可用的源）
      - --db-repository=ghcr.io/aquasecurity/trivy-db
      - --java-db-repository=ghcr.io/aquasecurity/trivy-java-db
    volumes:
      - trivy-cache:/root/.cache/trivy
    ports:
      - "4954:4954"
    restart: unless-stopped

volumes:
  trivy-cache:
    driver: local
```

**注意**：示例中使用的是官方源，如果有可用的国内镜像，请替换 URL。

### 方案 4: 离线环境部署

适用于无公网访问的内网环境，需预先下载漏洞数据库。

#### 步骤 1: 制作包含漏洞库的镜像

创建 `Dockerfile.trivy-server-offline`：

```dockerfile
FROM aquasec/trivy:latest

# 下载漏洞数据库到镜像中
RUN trivy image --download-db-only --cache-dir /root/.cache/trivy \
    && trivy image --download-java-db-only --cache-dir /root/.cache/trivy

# 启动 Trivy Server
ENTRYPOINT ["trivy"]
CMD ["server", "--listen=0.0.0.0:4954", "--skip-db-update=true"]
```

#### 步骤 2: 构建镜像

```bash
# 在有公网的机器上构建
docker build -f Dockerfile.trivy-server-offline -t trivy-server-offline:latest .

# 导出镜像
docker save trivy-server-offline:latest | gzip > trivy-server-offline.tar.gz
```

#### 步骤 3: 导入到离线环境

```bash
# 在离线环境导入镜像
docker load < trivy-server-offline.tar.gz
```

#### 步骤 4: 部署

```yaml
version: '3.8'

services:
  trivy-server:
    image: trivy-server-offline:latest
    container_name: trivy-server
    volumes:
      - trivy-cache:/root/.cache/trivy
    ports:
      - "4954:4954"
    restart: unless-stopped

volumes:
  trivy-cache:
    driver: local
```

**说明**：
- 镜像中已包含漏洞数据库，启动时设置 `--skip-db-update=true` 禁用更新
- 数据可能不是最新，建议定期重新构建镜像更新漏洞库
- 镜像大小约 200MB（包含数据库）

### 方案 5: Kubernetes 部署（生产环境）

适用于 Kubernetes 集群，提供高可用和自动扩展。

#### 创建 Deployment 和 Service

`trivy-server.yaml`:

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: trivy-cache-pvc
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 5Gi
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: trivy-server
  labels:
    app: trivy-server
spec:
  replicas: 1  # Trivy Server 建议单副本（共享缓存）
  selector:
    matchLabels:
      app: trivy-server
  template:
    metadata:
      labels:
        app: trivy-server
    spec:
      containers:
      - name: trivy
        image: aquasec/trivy:latest
        command:
          - server
          - --listen=0.0.0.0:4954
          - --skip-db-update=false
        ports:
        - containerPort: 4954
          name: http
        volumeMounts:
        - name: trivy-cache
          mountPath: /root/.cache/trivy
        resources:
          requests:
            memory: "512Mi"
            cpu: "500m"
          limits:
            memory: "2Gi"
            cpu: "2000m"
        livenessProbe:
          httpGet:
            path: /healthz
            port: 4954
          initialDelaySeconds: 60
          periodSeconds: 30
        readinessProbe:
          httpGet:
            path: /healthz
            port: 4954
          initialDelaySeconds: 10
          periodSeconds: 10
      volumes:
      - name: trivy-cache
        persistentVolumeClaim:
          claimName: trivy-cache-pvc
---
apiVersion: v1
kind: Service
metadata:
  name: trivy-server
  labels:
    app: trivy-server
spec:
  type: ClusterIP
  ports:
  - port: 4954
    targetPort: 4954
    protocol: TCP
    name: http
  selector:
    app: trivy-server
```

#### 部署到 Kubernetes

```bash
kubectl apply -f trivy-server.yaml
```

#### Web UI 配置

```yaml
# 在 Web UI 的环境变量中配置
environment:
  - TRIVY_TRIVY_SERVER=http://trivy-server.default.svc.cluster.local:4954
```

## 漏洞库管理

### 查看数据库版本

```bash
# 进入容器
docker exec -it trivy-server sh

# 查看数据库元数据
cat /root/.cache/trivy/db/metadata.json
```

输出示例：

```json
{
  "Version": 2,
  "NextUpdate": "2025-10-04T10:30:00Z",
  "UpdatedAt": "2025-10-03T10:30:00Z"
}
```

### 手动更新数据库

```bash
# 在容器内执行
trivy image --download-db-only --cache-dir /root/.cache/trivy
trivy image --download-java-db-only --cache-dir /root/.cache/trivy
```

或者重启 Trivy Server（如果启用了自动更新）：

```bash
docker-compose restart trivy-server
```

### 清理旧数据库

```bash
# 删除数据库缓存，强制重新下载
docker exec -it trivy-server sh -c "rm -rf /root/.cache/trivy/db/* /root/.cache/trivy/java-db/*"

# 重启服务触发重新下载
docker-compose restart trivy-server
```

## 验证部署

### 1. 检查 Trivy Server 健康状态

```bash
curl http://localhost:4954/healthz
```

预期输出：`OK`

### 2. 测试扫描功能

```bash
# 使用 trivy 命令行测试
docker exec -it trivy-server trivy image \
  --server http://localhost:4954 \
  --format json \
  alpine:latest
```

### 3. 检查 Web UI 连接

访问 Web UI（如 `http://localhost:8080`），提交一个扫描任务，观察日志：

```bash
docker-compose logs -f trivy-web-ui
```

应该能看到类似的日志：

```
[INFO] Trivy Configuration:
[INFO]   Server URL: http://trivy-server:4954
```

## 性能调优

### 资源建议

**Trivy Server**:
- **内存**: 512MB - 2GB（取决于扫描频率和镜像大小）
- **CPU**: 0.5 - 2 核
- **存储**: 5GB（持久化缓存卷）

**Web UI**:
- **内存**: 256MB - 512MB
- **CPU**: 0.25 - 1 核
- **存储**: 10GB（扫描报告存储）

### 并发控制

如果有多个 Web UI 实例，建议：

1. **Trivy Server 单副本**：共享漏洞数据库，避免重复下载
2. **Web UI 多副本**：根据负载水平扩展
3. **配置负载均衡**：使用 Nginx/Traefik 分发请求到 Web UI

示例架构：

```
Internet
   |
[Load Balancer]
   |
   +---> [Web UI 1] --\
   |                   \
   +---> [Web UI 2] ----+---> [Trivy Server]
   |                   /
   +---> [Web UI 3] --/
```

## 常见问题

### Q: Trivy Server 启动失败，提示数据库下载超时？

**A**: 首次启动需要下载漏洞数据库（约 100MB），可能需要几分钟。建议：
1. 检查网络连接，确保可以访问 `ghcr.io`
2. 增加健康检查的 `start_period` 时间
3. 使用国内镜像源加速
4. 或采用离线部署方案（预先下载数据库）

### Q: 如何升级漏洞数据库？

**A**: 如果启用了自动更新（`--skip-db-update=false`），Trivy Server 会每 12 小时自动检查更新。手动强制更新：

```bash
# 重启服务触发更新
docker-compose restart trivy-server

# 或进入容器手动下载
docker exec -it trivy-server trivy image --download-db-only
```

### Q: 多个 Web UI 实例能否共享同一个 Trivy Server？

**A**: 可以！这正是 Client-Server 模式的优势。所有 Web UI 实例配置相同的 `TRIVY_TRIVY_SERVER` 地址即可。

### Q: 离线环境如何定期更新漏洞库？

**A**: 建议定期（如每月）在有公网的机器上重新构建包含最新数据库的镜像，然后导入到离线环境。

### Q: Trivy Server 是否支持高可用部署？

**A**: Trivy Server 官方推荐单副本部署（共享缓存），如需高可用：
1. 使用共享存储（NFS/Ceph）挂载缓存目录
2. 部署多副本，配置 PVC `ReadWriteMany` 访问模式
3. 或使用外部缓存（Redis），但需自定义实现

## 参考资料

- [Trivy 官方文档](https://aquasecurity.github.io/trivy/)
- [Trivy Server 模式文档](https://aquasecurity.github.io/trivy/latest/docs/references/modes/client-server/)
- [Trivy 镜像仓库](https://github.com/aquasecurity/trivy)
