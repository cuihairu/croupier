---
title: Kubernetes Deployment
icon: cloud
order: 5
category:
  - Operations Manual
tag:
  - Deployment
  - kubernetes
---

# Kubernetes Deployment

> The repository does not yet ship an official Helm Chart / manifests; the following are **reference templates** — tailor them to your namespaces, image registry, and game network topology. The Server has no local persistent state and needs no PVC (except for SQLite deployments).

## Topology

```
Game pods ──► Agent Deployment ──outbound TCP──► Service(croupier-server:19090)
                                                ├─► server-0 (Deployment, 2 replicas)
                                                └─► server-1
Dashboard/UI ──► Ingress ──► Service(croupier-server:18780)
```

Key points:

- Agents make **outbound connections**; the game network only needs to allow egress to Service `:19090` — the Server does not need to be exposed into the game network
- kube-proxy spreads TCP long connections at L4 naturally, equivalent to the HAProxy role in [Load Balancing](./load-balancing); on disconnect, Agents automatically fail over to another backend on reconnect
- Both ports can live on the same Service; no session affinity needed

## ConfigMap and Secret

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

> The first-startup migration catch-up may leave readiness unready initially — give `initialDelaySeconds` enough time for the baseline AutoMigrate, or wrap it in a `startupProbe`.

## Service and Ingress

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

Ingress note: SSE (task event streams, config watch) requires disabling proxy buffering (nginx ingress annotation `nginx.ingress.kubernetes.io/proxy-buffering: "off"`).

## Agents (Game Network Side)

Agents are deployed in the same network as the game servers (Deployment or DaemonSet as needed); configuration goes through a ConfigMap + `--config` (this binary has no env-expansion layer; env overrides of the form `CROUPIER_AGENT_*` do not exist):

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

ConfigMap contents (`server.addr` points at the Service DNS; `agent.httpAddr` uses an address reachable by the Server):

```yaml
server:
  addr: "croupier-server.namespace.svc:19090"
agent:
  gameId: "demo" # Required
  env: "prod" # Required
  httpAddr: "croupier-agent.namespace.svc:19091"
```

When each Pod needs its own callback address (requires injecting `POD_IP`), use a StatefulSet to pin pod DNS names or write that address into the ConfigMap; this binary does not support expanding environment variables inside YAML values.

For cross-cluster / game IDC scenarios: replace the Service with a LoadBalancer or an entry point exposed over a dedicated line/VPN; everything else stays the same.

## Verification Checklist

```bash
kubectl get pods -l app=croupier-server        # 2/2 Running
curl http://<svc>:18780/healthz                # ok
curl http://<svc>:18780/api/v1                 # version metadata
kubectl logs deploy/croupier-server | grep -i "migrat\|cluster"   # migrations and member table
```

The Dashboard "Ops Center → Cluster Topology" should show 2 online members and their agent distribution.
