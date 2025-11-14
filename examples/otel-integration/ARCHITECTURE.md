# 🏗️ OpenTelemetry + Croupier 完整架构图

## 📊 数据流架构总览

```mermaid
graph TB
    subgraph "游戏客户端层"
        Game1[Unity游戏<br/>+SimpleAnalytics]
        Game2[Unreal游戏<br/>+SimpleAnalytics]
        Game3[服务器游戏<br/>+OTel SDK]
        Game4[H5游戏<br/>+JS SDK]
    end

    subgraph "数据收集层"
        LB[负载均衡器<br/>Nginx/HAProxy]
        Server1[Croupier Server 1<br/>HTTP API]
        Server2[Croupier Server 2<br/>HTTP API]
        Server3[Croupier Server N<br/>HTTP API]

        Collector1[OTel Collector 1<br/>OTLP接收器]
        Collector2[OTel Collector 2<br/>OTLP接收器]
        Bridge1[Analytics Bridge 1<br/>协议转换]
        Bridge2[Analytics Bridge 2<br/>协议转换]
    end

    subgraph "消息队列层"
        RedisCluster[Redis Cluster<br/>Streams MQ]
        RedisNode1[Redis Node 1<br/>analytics:events<br/>analytics:payments]
        RedisNode2[Redis Node 2<br/>analytics:events<br/>analytics:payments]
        RedisNode3[Redis Node 3<br/>analytics:events<br/>analytics:payments]

        RedisCluster --> RedisNode1
        RedisCluster --> RedisNode2
        RedisCluster --> RedisNode3
    end

    subgraph "数据处理层"
        WorkerGroup[Analytics Worker Group]
        Worker1[Worker 1<br/>Consumer Group: analytics-workers]
        Worker2[Worker 2<br/>Consumer Group: analytics-workers]
        Worker3[Worker 3<br/>Consumer Group: analytics-workers]
        WorkerN[Worker N<br/>Consumer Group: analytics-workers]

        WorkerGroup --> Worker1
        WorkerGroup --> Worker2
        WorkerGroup --> Worker3
        WorkerGroup --> WorkerN
    end

    subgraph "存储层"
        CHCluster[ClickHouse 集群]
        CH1[(ClickHouse 1<br/>analytics.events<br/>analytics.payments<br/>analytics.daily_users)]
        CH2[(ClickHouse 2<br/>Replica)]
        CH3[(ClickHouse 3<br/>Replica)]

        CHCluster --> CH1
        CHCluster --> CH2
        CHCluster --> CH3
    end

    subgraph "观测性层"
        Jaeger[Jaeger 分布式追踪]
        Prometheus[Prometheus 指标]
        Grafana[Grafana 可视化]
        AlertManager[AlertManager 告警]
    end

    subgraph "应用层"
        Dashboard[Croupier Dashboard<br/>游戏运营面板]
        API[Analytics API<br/>第三方集成]
    end

    %% 数据流连接
    Game1 -->|HTTP POST| LB
    Game2 -->|HTTP POST| LB
    Game3 -->|OTLP gRPC| Collector1
    Game4 -->|HTTP POST| LB

    LB --> Server1
    LB --> Server2
    LB --> Server3

    Server1 -->|events| RedisCluster
    Server2 -->|events| RedisCluster
    Server3 -->|events| RedisCluster

    Collector1 --> Bridge1
    Collector2 --> Bridge2
    Bridge1 -->|events| RedisCluster
    Bridge2 -->|events| RedisCluster

    RedisNode1 -->|stream consume| Worker1
    RedisNode2 -->|stream consume| Worker2
    RedisNode3 -->|stream consume| Worker3
    RedisNode1 -->|stream consume| WorkerN

    Worker1 -->|batch insert| CHCluster
    Worker2 -->|batch insert| CHCluster
    Worker3 -->|batch insert| CHCluster
    WorkerN -->|batch insert| CHCluster

    CHCluster --> Dashboard
    CHCluster --> API

    Collector1 -->|traces| Jaeger
    Collector1 -->|metrics| Prometheus
    Prometheus --> Grafana
    Jaeger --> Grafana
    Prometheus --> AlertManager

    classDef game fill:#e6f7ff,stroke:#1890ff
    classDef server fill:#f6ffed,stroke:#52c41a
    classDef storage fill:#fff7e6,stroke:#fa8c16
    classDef monitor fill:#f9f0ff,stroke:#722ed1
    classDef mq fill:#f0f9e6,stroke:#52c41a

    class Game1,Game2,Game3,Game4 game
    class Server1,Server2,Server3,Collector1,Collector2,Bridge1,Bridge2,Worker1,Worker2,Worker3,WorkerN,LB server
    class CH1,CH2,CH3,CHCluster storage
    class Jaeger,Prometheus,Grafana,AlertManager monitor
    class RedisCluster,RedisNode1,RedisNode2,RedisNode3 mq
```

## 🔄 消息队列详细架构

```mermaid
graph LR
    subgraph "Producer Layer"
        P1[Croupier Server 1]
        P2[Croupier Server 2]
        P3[OTel Bridge 1]
        P4[OTel Bridge 2]
    end

    subgraph "Redis Streams Cluster"
        subgraph "Shard 1: Events"
            Stream1[analytics:events<br/>MAXLEN ~1000000]
            CG1[Consumer Group:<br/>analytics-workers]
        end

        subgraph "Shard 2: Payments"
            Stream2[analytics:payments<br/>MAXLEN ~100000]
            CG2[Consumer Group:<br/>analytics-workers]
        end

        subgraph "Shard 3: Custom"
            Stream3[analytics:custom<br/>MAXLEN ~500000]
            CG3[Consumer Group:<br/>analytics-workers]
        end
    end

    subgraph "Consumer Group"
        C1[Worker-1<br/>Consumer: c-001]
        C2[Worker-2<br/>Consumer: c-002]
        C3[Worker-3<br/>Consumer: c-003]
        CN[Worker-N<br/>Consumer: c-xxx]

        subgraph "Processing"
            Batch[批量处理<br/>200 msgs/batch]
            Agg[实时聚合<br/>HyperLogLog]
            Store[批量写入<br/>ClickHouse]
        end
    end

    P1 -->|XADD events| Stream1
    P1 -->|XADD payments| Stream2
    P2 -->|XADD events| Stream1
    P2 -->|XADD payments| Stream2
    P3 -->|XADD events| Stream1
    P4 -->|XADD custom| Stream3

    CG1 -->|XREADGROUP| C1
    CG1 -->|XREADGROUP| C2
    CG2 -->|XREADGROUP| C2
    CG2 -->|XREADGROUP| C3
    CG3 -->|XREADGROUP| C3
    CG3 -->|XREADGROUP| CN

    C1 --> Batch
    C2 --> Batch
    C3 --> Batch
    CN --> Batch

    Batch --> Agg
    Agg --> Store

    classDef producer fill:#e6f7ff,stroke:#1890ff
    classDef stream fill:#f0f9e6,stroke:#52c41a
    classDef consumer fill:#fff7e6,stroke:#fa8c16
    classDef process fill:#f9f0ff,stroke:#722ed1

    class P1,P2,P3,P4 producer
    class Stream1,Stream2,Stream3,CG1,CG2,CG3 stream
    class C1,C2,C3,CN consumer
    class Batch,Agg,Store process
```

## ⚙️ 扩容和容错设计

```mermaid
graph TB
    subgraph "负载均衡层"
        direction TB
        ALB[Application Load Balancer<br/>支持健康检查]
        NLB[Network Load Balancer<br/>TCP层负载均衡]
    end

    subgraph "应用层高可用"
        direction LR
        subgraph "Region A"
            ServerA1[Croupier Server A1<br/>Primary]
            ServerA2[Croupier Server A2<br/>Primary]
            CollectorA1[OTel Collector A1]
            CollectorA2[OTel Collector A2]
        end

        subgraph "Region B (DR)"
            ServerB1[Croupier Server B1<br/>Standby]
            CollectorB1[OTel Collector B1]
        end
    end

    subgraph "消息队列高可用"
        direction LR
        subgraph "Redis Cluster - Master/Slave"
            subgraph "Master Nodes"
                RM1[Redis Master 1<br/>Slot: 0-5461]
                RM2[Redis Master 2<br/>Slot: 5462-10922]
                RM3[Redis Master 3<br/>Slot: 10923-16383]
            end

            subgraph "Slave Nodes"
                RS1[Redis Slave 1<br/>Replica of M1]
                RS2[Redis Slave 2<br/>Replica of M2]
                RS3[Redis Slave 3<br/>Replica of M3]
            end
        end

        subgraph "Sentinel Monitor"
            Sentinel1[Redis Sentinel 1]
            Sentinel2[Redis Sentinel 2]
            Sentinel3[Redis Sentinel 3]
        end
    end

    subgraph "处理层自动扩容"
        direction TB
        subgraph "Kubernetes Deployment"
            HPA[HPA Controller<br/>基于队列长度扩容]
            WorkerDeployment[Analytics Worker<br/>Deployment: 3-20 Pods]

            subgraph "Worker Pods"
                WP1[Worker Pod 1<br/>Consumer: pod-1]
                WP2[Worker Pod 2<br/>Consumer: pod-2]
                WP3[Worker Pod 3<br/>Consumer: pod-3]
                WPN[Worker Pod N<br/>Consumer: pod-N]
            end
        end
    end

    subgraph "存储层高可用"
        direction LR
        subgraph "ClickHouse Cluster"
            subgraph "Shard 1"
                CH1A[ClickHouse 1A<br/>Replica 1]
                CH1B[ClickHouse 1B<br/>Replica 2]
            end

            subgraph "Shard 2"
                CH2A[ClickHouse 2A<br/>Replica 1]
                CH2B[ClickHouse 2B<br/>Replica 2]
            end

            ZK[ZooKeeper Ensemble<br/>Coordination]
        end
    end

    %% 连接关系
    ALB --> ServerA1
    ALB --> ServerA2
    NLB --> CollectorA1
    NLB --> CollectorA2

    ServerA1 --> RM1
    ServerA2 --> RM2
    CollectorA1 --> RM1
    CollectorA2 --> RM3

    RM1 --> RS1
    RM2 --> RS2
    RM3 --> RS3

    Sentinel1 -.->|monitor| RM1
    Sentinel2 -.->|monitor| RM2
    Sentinel3 -.->|monitor| RM3

    RM1 --> WP1
    RM2 --> WP2
    RM3 --> WP3
    RM1 --> WPN

    HPA -.->|scale| WorkerDeployment
    WorkerDeployment --> WP1
    WorkerDeployment --> WP2
    WorkerDeployment --> WP3
    WorkerDeployment --> WPN

    WP1 --> CH1A
    WP2 --> CH2A
    WP3 --> CH1B
    WPN --> CH2B

    ZK -.->|coordinate| CH1A
    ZK -.->|coordinate| CH1B
    ZK -.->|coordinate| CH2A
    ZK -.->|coordinate| CH2B

    classDef lb fill:#e6f7ff,stroke:#1890ff
    classDef app fill:#f6ffed,stroke:#52c41a
    classDef redis fill:#f0f9e6,stroke:#52c41a
    classDef worker fill:#fff7e6,stroke:#fa8c16
    classDef storage fill:#f9f0ff,stroke:#722ed1

    class ALB,NLB lb
    class ServerA1,ServerA2,ServerB1,CollectorA1,CollectorA2,CollectorB1 app
    class RM1,RM2,RM3,RS1,RS2,RS3,Sentinel1,Sentinel2,Sentinel3 redis
    class HPA,WorkerDeployment,WP1,WP2,WP3,WPN worker
    class CH1A,CH1B,CH2A,CH2B,ZK storage
```

## 📊 监控和告警架构

```mermaid
graph TB
    subgraph "数据源层"
        Metrics[Prometheus Metrics<br/>应用指标、系统指标]
        Traces[Jaeger Traces<br/>分布式追踪]
        Logs[日志聚合<br/>ELK/Loki Stack]
        Business[业务数据<br/>ClickHouse Analytics]
    end

    subgraph "监控层"
        Prometheus[Prometheus<br/>指标存储和查询]
        Grafana[Grafana<br/>可视化面板]
        AlertManager[AlertManager<br/>告警管理器]

        subgraph "自定义面板"
            GameDashboard[游戏运营面板<br/>DAU/Revenue/Retention]
            TechDashboard[技术监控面板<br/>QPS/Latency/Errors]
            BizDashboard[业务分析面板<br/>漏斗/留存/LTV]
        end
    end

    subgraph "告警渠道"
        Slack[Slack 通知]
        Email[邮件告警]
        SMS[短信告警]
        Webhook[Webhook 回调]
        PagerDuty[PagerDuty 值班]
    end

    subgraph "告警规则"
        subgraph "系统告警"
            SysAlerts[
            • Redis队列积压 > 10000
            • Worker处理延迟 > 30s
            • ClickHouse写入失败率 > 1%
            • OTel Collector丢包率 > 0.1%
            ]
        end

        subgraph "业务告警"
            BizAlerts[
            • DAU跌幅 > 10%
            • 付费转化率 < 阈值
            • 关卡通过率异常
            • 新用户留存异常
            ]
        end

        subgraph "性能告警"
            PerfAlerts[
            • API响应时间 > 2s
            • 错误率 > 5%
            • CPU/内存使用 > 80%
            • 磁盘空间 < 20%
            ]
        end
    end

    %% 数据流连接
    Metrics --> Prometheus
    Traces --> Grafana
    Logs --> Grafana
    Business --> GameDashboard

    Prometheus --> Grafana
    Prometheus --> AlertManager

    Grafana --> GameDashboard
    Grafana --> TechDashboard
    Grafana --> BizDashboard

    AlertManager --> Slack
    AlertManager --> Email
    AlertManager --> SMS
    AlertManager --> Webhook
    AlertManager --> PagerDuty

    SysAlerts --> AlertManager
    BizAlerts --> AlertManager
    PerfAlerts --> AlertManager

    classDef source fill:#e6f7ff,stroke:#1890ff
    classDef monitor fill:#f6ffed,stroke:#52c41a
    classDef alert fill:#fff2e8,stroke:#fa541c
    classDef rules fill:#f9f0ff,stroke:#722ed1

    class Metrics,Traces,Logs,Business source
    class Prometheus,Grafana,AlertManager,GameDashboard,TechDashboard,BizDashboard monitor
    class Slack,Email,SMS,Webhook,PagerDuty alert
    class SysAlerts,BizAlerts,PerfAlerts rules
```

## 🔐 安全架构

```mermaid
graph TB
    subgraph "网络安全层"
        WAF[Web应用防火墙<br/>SQL注入/XSS防护]
        CDN[CDN + DDoS防护<br/>CloudFlare/AWS Shield]
        VPN[VPN网关<br/>内网访问控制]
    end

    subgraph "认证授权层"
        OAuth[OAuth 2.0 / OIDC<br/>统一身份认证]
        RBAC[RBAC权限控制<br/>角色基础访问控制]
        JWT[JWT Token<br/>API访问令牌]
        mTLS[mTLS证书<br/>服务间通信加密]
    end

    subgraph "数据安全层"
        Encryption[数据加密<br/>传输加密(TLS) + 存储加密(AES)]
        Anonymization[数据脱敏<br/>PII数据匿名化]
        Backup[数据备份<br/>增量备份 + 异地容灾]
        Audit[审计日志<br/>操作审计 + 数据访问审计]
    end

    subgraph "运维安全层"
        Secrets[密钥管理<br/>Vault/K8s Secrets]
        SIEM[安全信息事件管理<br/>异常行为检测]
        Compliance[合规检查<br/>GDPR/SOC2合规]
        Monitoring[安全监控<br/>入侵检测 + 威胁情报]
    end

    CDN --> WAF
    WAF --> OAuth
    OAuth --> RBAC
    RBAC --> JWT
    JWT --> mTLS

    mTLS --> Encryption
    Encryption --> Anonymization
    Anonymization --> Backup
    Backup --> Audit

    Audit --> Secrets
    Secrets --> SIEM
    SIEM --> Compliance
    Compliance --> Monitoring

    classDef network fill:#e6f7ff,stroke:#1890ff
    classDef auth fill:#f6ffed,stroke:#52c41a
    classDef data fill:#fff7e6,stroke:#fa8c16
    classDef ops fill:#f9f0ff,stroke:#722ed1

    class WAF,CDN,VPN network
    class OAuth,RBAC,JWT,mTLS auth
    class Encryption,Anonymization,Backup,Audit data
    class Secrets,SIEM,Compliance,Monitoring ops
```

---

*这些架构图展示了从简单到复杂的完整OTel+Croupier集成方案，涵盖了数据流、扩容、监控和安全等各个方面。*