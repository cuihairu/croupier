---
title: Kubernetes 部署
icon: cloud
order: 5
category:
  - 运维手册
tag:
  - 部署
  - kubernetes
---

# Kubernetes 部署

> 仓库尚未随附官方 Helm Chart / manifests；以下是**参考模板**——请按你的命名空间、镜像仓库与游戏网络拓扑裁剪。Server 无本地持久状态，不需要 PVC（SQLite 部署除外）。

## 拓扑

```
Game pods ──► Agent Deployment ──outbound TCP──► Service(croupier-server:19090)
                                                ├─► server-0 (Deployment, 2 replicas)
                                                └─► server-1
Dashboard/UI ──► Ingress ──► Service(croupier-server:18780)
```

要点：

- Agent 是**外连**发起方；游戏网络只需放行到 Service `:19090` 的出方向——Server 不需要暴露进游戏网络
- 两个端口可以共用同一个 Service；无需会话亲和

## ConfigMap 与 Secret

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: croupier-server-config
data:
  server.yaml: |
    server: { host: 0.0.0.0, port: 18780, mode: prod }
    database:
      driver: mysql
      dataSource: "user:$(DB_PASSWORD)@tcp(mysql:3306)/croupier_meta?charset=utf8mb4&parseTime=True&loc=Local"
    auth:
      jwtSecret: "$(CROUPIER_JWT_SECRET)"   # Expanded from environment variables at load time (ExpandEnv)
    cache: { enabled: true, type: redis, addr: redis:6379 }
    # Required for multi-instance
    cluster:
      enabled: true
      advertiseAddr: ""   # When empty, POD_IP is used (see the env var injection below)
---
apiVersion: v1
kind: Secret
metadata:
  name: croupier-server-secret
stringData:
  jwt-secret: "change-me"
```

## Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: croupier-server
spec:
  replicas: 2
  strategy:
    rollingUpdate: { maxSurge: 1, maxUnavailable: 0 } # Rolling upgrade without interrupting traffic
  selector:
    matchLabels: { app: croupier-server }
  template:
    metadata:
      labels: { app: croupier-server }
    spec:
      containers:
        - name: server
          image: your-registry/croupier-server:v0.1.1
          args: ["--config", "/etc/croupier/server.yaml"]
          ports:
            - { name: http, containerPort: 18780 }
            - { name: transport, containerPort: 19090 }
          env:
            - name: POD_IP
              valueFrom: { fieldRef: { fieldPath: status.podIP } }
            - name: CROUPIER_CLUSTER_ADVERTISE_ADDR # Override variable read explicitly by root.go: Pod IP for instance interconnect
              value: "$(POD_IP):19099"
            - name: CROUPIER_JWT_SECRET # Expands the $(CROUPIER_JWT_SECRET) placeholder in server.yaml
              valueFrom:
                {
                  secretKeyRef:
                    { name: croupier-server-secret, key: jwt-secret },
                }
          readinessProbe:
            httpGet: { path: /healthz, port: http }
            initialDelaySeconds: 5
            periodSeconds: 10
          livenessProbe:
            httpGet: { path: /healthz, port: http }
            initialDelaySeconds: 20
            periodSeconds: 20
          volumeMounts:
            - { name: config, mountPath: /etc/croupier, readOnly: true }
      volumes:
        - name: config
          configMap: { name: croupier-server-config }
```

> 首次启动的迁移追平可能让 readiness 一度不 ready——`initialDelaySeconds` 给基线 AutoMigrate 留足时间，或包一层 `startupProbe`。

## Service 与 Ingress

```yaml
apiVersion: v1
kind: Service
metadata:
  name: croupier-server
spec:
  selector: { app: croupier-server }
  ports:
    - { name: http, port: 18780, targetPort: http }
    - { name: transport, port: 19090, targetPort: transport }
---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: croupier-dashboard
spec:
  rules:
    - host: croupier.example.com
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              { service: { name: croupier-server, port: { number: 18780 } } }
```

Ingress 注意：SSE（任务事件流、配置 watch）需要关闭代理缓冲（nginx ingress 注解 `nginx.ingress.kubernetes.io/proxy-buffering: "off"`）。

## Agent（游戏网络侧）

Agent 与游戏服同网络部署（Deployment 或 DaemonSet 按需）；配置走 ConfigMap + `--config`（该二进制没有 env 展开层，不存在 `CROUPIER_AGENT_*` 形态的环境变量覆盖）：

```yaml
command: ["agent", "--config", "/etc/croupier/agent.yaml"]
volumeMounts:
  - name: config
    mountPath: /etc/croupier
volumes:
  - name: config
    configMap:
      name: croupier-agent
```

ConfigMap 内容（`server.addr` 指向 Service DNS；`agent.httpAddr` 用 Server 可达地址）：

```yaml
server:
  addr: "croupier-server.namespace.svc:19090"
agent:
  gameId: "demo" # Required
  env: "prod" # Required
  httpAddr: "croupier-agent.namespace.svc:19091"
```

每个 Pod 需要独立回调地址时（需注入 `POD_IP`），用 StatefulSet 固定 pod DNS 名或把该地址写进 ConfigMap；该二进制不支持在 YAML 值里展开环境变量。

跨集群/游戏 IDC 场景：把 Service 换成 LoadBalancer 或专线/VPN 暴露的入口，其余不变。

## 验证清单

```bash
kubectl get pods -l app=croupier-server        # 2/2 Running
curl http://<svc>:18780/healthz                # ok
curl http://<svc>:18780/api/v1                 # version metadata
kubectl logs deploy/croupier-server | grep -i "migrat\|cluster"   # migrations and member table
```

Dashboard「运营中心 → 集群拓扑」应能看到 2 个在线成员及其 agent 分布。
