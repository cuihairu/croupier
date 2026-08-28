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

> 仓库暂未附带官方 Helm Chart / manifests，以下为**参考模板**——按命名空间、镜像仓库与游戏网络拓扑裁剪。Server 无本地持久状态，不需要 PVC（SQLite 部署除外）。

## 拓扑

```
Game pods ──► Agent Deployment ──出站 TCP──► Service(croupier-server:19090)
                                                ├─► server-0 (Deployment, 2 副本)
                                                └─► server-1
Dashboard/UI ──► Ingress ──► Service(croupier-server:18780)
```

要点：

- Agent 是**出站连接**，游戏网络只需允许 egress 到 Service `:19090`——不需要把 Server 暴露进游戏网络
- kube-proxy 对 TCP 长连接做天然 L4 打散，等价于[负载均衡](./load-balancing)中的 HAProxy 职能；Agent 断线重连自动换后端
- 双端口同一 Service 即可；无需 session 亲和

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
      jwtSecret: "$(CROUPIER_JWT_SECRET)"   # 加载时按环境变量展开（ExpandEnv）
    cache: { enabled: true, type: redis, addr: redis:6379 }
    # 多实例必开
    cluster:
      enabled: true
      advertiseAddr: ""   # 留空时使用 POD_IP（见下方环境变量注入）
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
    rollingUpdate: { maxSurge: 1, maxUnavailable: 0 } # 滚动升级不断流
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
            - name: CROUPIER_CLUSTER_ADVERTISE_ADDR # root.go 显式读取的覆盖变量：Pod IP 做实例互联
              value: "$(POD_IP):19099"
            - name: CROUPIER_JWT_SECRET # 供 server.yaml 占位符 $(CROUPIER_JWT_SECRET) 展开
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

> 首次启动的迁移 catch-up 可能在 readiness 初期未就绪——`initialDelaySeconds` 给足基线 AutoMigrate 时间，或用 `startupProbe` 包住。

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

Agent 与游戏服同网络部署（Deployment 或 DaemonSet 按需），`server.addr` 指向 Service DNS：

```yaml
env:
  - name: CROUPIER_AGENT_SERVER_ADDR
    value: "croupier-server.namespace.svc:19090"
  - name: CROUPIER_AGENT_AGENT_HTTPADDR
    value: "$(POD_IP):19091" # 对 Server 可达的回调地址
```

跨集群/游戏 IDC 场景：Service 换成 LoadBalancer 或经专线/VPN 暴露的入口，其余不变。

## 验证清单

```bash
kubectl get pods -l app=croupier-server        # 2/2 Running
curl http://<svc>:18780/healthz                # ok
curl http://<svc>:18780/api/v1                 # 版本元信息
kubectl logs deploy/croupier-server | grep -i "migrat\|cluster"   # 迁移与成员表
```

Dashboard「运维中心 → 集群拓扑」应显示 2 个在线成员及其 agent 分布。
