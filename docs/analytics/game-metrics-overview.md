# Game Metrics Panorama

This document provides the complete game data metrics system, covering every stage from user acquisition to long-term operations.

## Metrics Classification System

### 1. Foundation Metrics Layer

> **Scope**: core metrics every game type must collect

#### 1.1 User Activity and Retention

| Metric | Definition | Formula | Data source | Frequency |
| ----------- | ------------ ----------------------------------- | -------------- | -------- |
| **DAU** | Daily active users | Unique users with game activity that day | Server login logs | Real-time |
| **MAU** | Monthly active users | Unique users with game activity that month | Server login logs | Daily |
| **WAU** | Weekly active users | Unique users with game activity that week | Server login logs | Daily |
| **D1 retention** | Next-day retention rate | Users logging in on day 2 after registration / registered users | User behavior logs | Daily |
| **D7 retention** | 7-day retention rate | Users logging in on day 8 after registration / registered users | User behavior logs | Weekly |
| **D30 retention** | 30-day retention rate | Users logging in on day 31 after registration / registered users | User behavior logs | Monthly |
| **DAU/MAU** | User engagement | DAU / MAU * 100% | Computed metric | Daily |

#### 1.2 Session Metrics

| Metric                     | Definition                 | Formula                         | Data source     | Business value           |
| -------------------------- | -------------------------- | ------------------------------- | --------------- | ------------------------ |
| **Average session length** | Single play session length | Total play time / session count | Client + server | Stickiness measure       |
| **Session frequency**      | Average sessions per day   | Total sessions / DAU            | Server logs     | Usage frequency analysis |
| **Session depth**          | Actions per session        | Actions / sessions              | Client events   | Engagement analysis      |

#### 1.3 Technical Quality Metrics

| Metric | Definition | Formula | Data source | Alert threshold |
| ------------ | ------------ --------------------------- | ---------- | ------------- |
| **Crash rate** | Application crash frequency | Crashed sessions / total sessions | Client SDK | > 1% |
| **ANR rate** | Application not-responding rate | ANR count / launch count | Client SDK | > 0.5% |
| **Load time** | Application startup time | Launch-complete timestamp - launch timestamp | Client SDK | P95 > 5s |
| **Frame rate** | Runtime frame rate | Average FPS | Client monitoring | &lt; 30fps |
| **Memory usage** | Game memory footprint | Peak memory usage | Client monitoring | > 80% of system memory |

### 2. Monetization Metrics Layer

> **Scope**: monetization analysis for paid and F2P games

#### 2.1 Payment Conversion Metrics

| Metric    | Definition                      | Formula                                         | Data source          | Key analysis dimensions       |
| --------- | ------------------------------- | ----------------------------------------------- | -------------------- | ----------------------------- |
| **ARPU**  | Average revenue per user        | Total revenue / DAU                             | Server payment logs  | Time, channel, region         |
| **ARPPU** | Average revenue per paying user | Total revenue / paying users                    | Server payment logs  | Payment tier, game mode       |
| **PUR**   | Payer rate                      | Paying users / DAU * 100%                       | Server payment logs  | New vs existing users, tenure |
| **LTV**   | User lifetime value             | Cumulative revenue / users                      | ML prediction models | Cohort analysis               |
| **ROI**   | Return on investment            | (Revenue - acquisition cost) / acquisition cost | Ads + revenue data   | Channel, campaign             |

#### 2.2 Acquisition Cost Metrics

| Metric | Definition | Formula | Data source | Key scenarios |
| ----------- | ------------ ---------------------- | ------------ | ------------ |
| **CPI** | Cost per install | Ad spend / installs | Ad platform API | Channel optimization |
| **CPA** | Cost per acquisition | Ad spend / registrations | Ad platform API + registration data | Channel evaluation |
| **CAC** | Customer acquisition cost | Ad spend / paying users | Ad platform API + payment data | Monetization strategy |
| **LTV/CAC** | Value-to-cost ratio | LTV / CAC | Computed metric | Channel investment decisions |

#### 2.3 Item Analysis Metrics

| Metric | Definition | Formula | Data source | Use case |
| ---------------- | ----------------------------- | --------------------- | ------------ |
| **Item conversion rate** | Single-item purchase conversion | Purchases / views | Client behavior + payment logs | Item optimization |
| **Average order value** | Average spend per order | Total revenue / order count | Server payment logs | Pricing strategy |
| **Repeat purchase rate** | Repeat purchase rate | Multi-purchase users / total paying users | Server payment logs | User value analysis |
| **Item lifecycle** | Item popularity duration | Days for sales to fall to 50% of peak | Time-series analysis | Item planning |

### 3. Content Quality Metrics Layer

> **Scope**: games with explicit content progression (levels, quest systems, etc.)

#### 3.1 Content Consumption Metrics

| Metric | Definition | Formula | Data source | Design value |
| ---------------- | ------------ ------------------------ | -------------- | ------------ |
| **Level completion rate** | Level clear ratio | Users completed / users started | Server progress logs | Difficulty balancing |
| **Average clear time** | Time to clear a level | Total clear time / clear count | Server progress logs | Duration design |
| **Retry distribution** | Level retry statistics | User share per retry-count bucket | Server progress logs | Frustration control |
| **Skip rate** | Content skip ratio | Skips / total attempts | Client events | Content appeal |
| **Content consumption speed** | Progression speed | Levels cleared / days played | Server progress logs | Content supply pacing |

#### 3.2 Quest System Metrics

| Metric | Definition | Formula | Data source | Optimization direction |
| ---------------- | -------------- ------------------- | -------------- | ------------ |
| **Quest completion rate** | Quest system engagement | Quests completed / quests accepted | Server quest logs | Quest design |
| **Daily quest activity** | Daily participation | Users completing daily quests / DAU | Server quest logs | DAU growth |
| **Achievement unlock rate** | Achievement system effect | Achievements unlocked / total achievements | Server achievement logs | Long-term goal design |

### 4. Social & Competitive Metrics Layer

> **Scope**: multiplayer, competitive, and social games

#### 4.1 Social Behavior Metrics

| Metric | Definition | Formula | Data source | Social value |
| -------------- | ------------------------- | -------------- | ------------ |
| **Friend-count distribution** | User social network size | User share per friend-count bucket | Server social logs | Social design |
| **Guild participation rate** | Guild system activity | Guild members / total users | Server guild logs | Organization depth |
| **Chat activity** | Social interaction frequency | Messages sent / DAU | Server chat logs | Community climate |
| **Gifting rate** | Social reciprocity behavior | Gifts sent / DAU | Server social logs | Social stickiness |
| **Party frequency** | Co-op participation | Party games / total games | Server party logs | Co-op mechanism effect |

#### 4.2 Competitive Balance Metrics

| Metric | Definition | Formula | Data source | Balance value |
| ------------ | -------------- --------------------- | -------------- | ------------ |
| **Win-rate distribution** | Competitive outcome distribution | User share per win-rate bucket | Server battle logs | Matchmaking optimization |
| **Character win rate** | Balance across characters | Per-character win-rate statistics | Server battle logs | Numeric balance tuning |
| **Rank distribution** | Skill-level distribution | User share per tier | Server ranking data | Promotion design |
| **Match quality** | Match balance | Share of matches with skill gap &lt;= 5% | Server matching logs | Matchmaking optimization |

#### 4.3 PVP/PVE Metrics

| Metric | Definition | Formula | Data source | Design guidance |
| -------------- | -------------- ------------------------ | ------------------ | ------------ |
| **PVP participation rate** | Competitive mode participation | PVP games / total games | Server game-mode logs | Competitive content planning |
| **PVE preference** | Solo mode preference | PVE playtime / total playtime | Server game-mode logs | Content ratio allocation |
| **Cross-mode conversion** | Movement between modes | PVE→PVP converted users | User behavior analysis | Guidance design |

### 5. Campaign & Operations Metrics Layer

> **Scope**: games with live operations events

#### 5.1 Event Effectiveness Metrics

| Metric | Definition | Formula | Data source | Ops value |
| ---------------- | ------------------ --------------------------------- | -------------- | ------------ |
| **Event participation rate** | Share of users covered by the event | Event participants / DAU | Server event logs | Event design review |
| **Event completion rate** | Event quest completion | Users completing the event / event participants | Server event logs | Event difficulty tuning |
| **Event revenue** | Incremental revenue from the event | Event-period revenue - baseline revenue | Revenue attribution analysis | ROI calculation |
| **Event retention impact** | Event's effect on retention | Participant retention - non-participant retention | User behavior comparison | Event effectiveness review |

#### 5.2 Push Marketing Metrics

| Metric | Definition | Formula | Data source | Optimization direction |
| -------------- | ---------------------- -------------------- | ------------ | ------------ |
| **Push delivery rate** | Push message arrival rate | Delivered / sent | Push platform API | Push channel optimization |
| **Push open rate** | Push message click-through rate | Opened / delivered | Push platform API + client | Content optimization |
| **Push conversion rate** | Behavior conversion driven by pushes | Target actions completed / push opens | Behavior attribution analysis | Push strategy optimization |
| **Push fatigue** | Declining user receptiveness to pushes | Declining open-rate trend of consecutive pushes | Time-series analysis | Push frequency control |

### 6. User Experience Metrics Layer

> **Scope**: games that prioritize user experience

#### 6.1 UI Interaction Metrics

| Metric | Definition | Formula | Data source | UX optimization value |
| ---------------- | ---------------------- --------------------------- | -------------- | ------------ |
| **Page dwell time** | Time distribution across pages | Page exit time - page enter time | Client page tracking | Information architecture optimization |
| **Button click rate** | UI element interaction frequency | Clicks / impressions | Client UI events | Interface design optimization |
| **Operation error rate** | User error frequency | Wrong operations / total operations | Client interaction events | Usability improvement |
| **Feature discovery rate** | Share of users discovering new features | New-feature users / total users | Client feature usage | Onboarding design optimization |

#### 6.2 Performance Experience Metrics

| Metric | Definition | Formula | Data source | Performance optimization value |
| ---------------- | ------------------ --------------------------- | -------------- | ------------ |
| **Response latency** | Operation response time | Response complete time - operation start time | Client performance monitoring | Interaction experience optimization |
| **Network quality** | Connection stability | Connection success rate, latency distribution | Network monitoring SDK | Network architecture optimization |
| **Resource load time** | Game asset loading efficiency | Asset download completion time | Client resource management | Asset optimization strategy |

### 7. Advanced Insights Metrics Layer

> **Scope**: mature games needing deep analysis and prediction

#### 7.1 Predictive Metrics

| Metric | Definition | Model | Data source | Prediction value |
| ------------------- | ------------------ ----------------------- | ------------ | ------------ |
| **Churn risk score** | Churn likelihood prediction | Machine learning classification model | User behavior features | Churn intervention |
| **LTV prediction** | Future user value estimate | Time-series forecasting model | Historical payment behavior | Segmented user operations |
| **Viral coefficient (K-factor)** | User propagation effect | Successful invites / active users | Social sharing data | Growth strategy optimization |

#### 7.2 User Segmentation Metrics

| Metric | Definition | Method | Data source | Ops application |
| ---------------- | ------------------ -------------------------- | ----------------- | ---------- |
| **User value tiering** | RFM model user segmentation | Recency, frequency, monetary clustering | User behavior + payment data | Precision marketing |
| **Game preference analysis** | User play behavior patterns | Cluster analysis | Game behavior logs | Personalized recommendations |
| **Lifecycle stage** | The user's stage in the game | Rules engine + behavior features | Consolidated user data | Stage-based operations |

#### 7.3 Market Insight Metrics

| Metric | Definition | Method | Data source | Business value |
| --------------- | -------------------- ------------------ | -------------- | ------------ |
| **NPS (Net Promoter Score)** | User recommendation willingness | (Promoters% - Detractors%) * 100 | User survey data | Brand health |
| **User satisfaction** | User satisfaction with the game | Surveys + behavior data | Multi-dimensional user feedback | Product improvement direction |
| **Competitor comparison** | Relative market position | Third-party comparative analysis | External data sources | Strategic decision support |

## Metrics Fit Matrix by Game Type

### Casual Games

**Core focus**: fast onboarding, high-frequency use, ad monetization

| Priority | Metric category | Key metrics | Special considerations |
| ------ | -------- ---------------------------- | -------------- |
| P0 | Retention & conversion | D1/D3/D7 retention, session length | Fragmented time distribution |
| P0 | Ad monetization | Ad show rate, eCPM, ad fatigue | Balancing user experience |
| P1 | Content consumption | Level completion rate, retry count | Onboarding difficulty curve |
| P2 | Social virality | Share rate, invite conversion | Viral mechanics |

### Heavy RPG

**Core focus**: deep progression, long-term payment, social interaction

| Priority | Metric category | Key metrics | Special considerations |
| ------ | -------- --------------------- | ------------ |
| P0 | Payment depth | ARPU, ARPPU, LTV | Payment path analysis |
| P0 | Growth system | Leveling speed, numeric balance | Growth satisfaction |
| P1 | Social stickiness | Guild activity, party frequency | Social dependency |
| P2 | Content consumption | Dungeon clear time, PVE preference | Content production pacing |

### Competitive Games

**Core focus**: fair competition, skill growth, the esports ecosystem

| Priority | Metric category | Key metrics | Special considerations |
| ------ | -------- ------------------ | ---------- |
| P0 | Competitive balance | Win-rate distribution, character balance | Fairness maintenance |
| P0 | Match quality | Matchmaking time, skill gap | Competitive experience |
| P1 | Skill growth | Rank promotion, mechanical improvement | Achievement design |
| P2 | Spectator ecosystem | Spectator watch time, share spread | Esports value |

### Strategy Games

**Core focus**: strategic depth, decision analysis, META evolution

| Priority | Metric category | Key metrics | Special considerations |
| ------ | -------- -------------------- | -------------- |
| P0 | Strategic depth | Decision branches, strategy success rate | Strategy diversity |
| P0 | Time investment | Thinking time, per-match length | Complexity balance |
| P1 | Learning curve | Advanced strategy adoption rate | Newcomer friendliness |
| P2 | META analysis | Popular strategy drift trends | Patch balance tuning |

## Metrics Implementation Priority Recommendations

### Phase 1: Foundational Data Build-Out (1-2 months)

**Goal**: establish the data collection foundation, covering core business metrics

- [**User activity metrics**: DAU/MAU, retention rates]
- [**Technical quality metrics**: crash rate, performance metrics]
- [**Basic monetization metrics**: revenue, payer rate]
- [**Basic content metrics**: level completion rate]

### Phase 2: Deepening Analysis Capability (2-4 months)

**Goal**: add analytical dimensions, support fine-grained operations

- [**Multi-dimensional breakdown**: channel, region, device analysis]
- [**User segmentation**: RFM analysis, behavior clustering]
- [**Live ops events**: event effectiveness, push analysis]
- [**Competitive & social**: PVP analysis, social networks]

### Phase 3: Intelligent Insights (4-6 months)

**Goal**: AI-driven predictive analysis and automated decisions

- [**Prediction models**: churn early warning, LTV prediction]
- [**Recommendation systems**: personalized content recommendations]
- [**Automatic optimization**: dynamic balancing, smart pushes]
- [**Competitive analysis**: market position analysis]

## Implementation Recommendations

### Data Quality Assurance

1. **Data consistency**: unified definitions, no metric ambiguity
2. **Data accuracy**: multi-source verification, outlier detection
3. **Data timeliness**: real-time computation for key metrics, T+1 batch processing for deep analysis
4. **Data completeness**: missing-value handling, data lineage management

### Performance Optimization Strategy

1. **Tiered storage**: hot data in a Redis cache, cold data in ClickHouse
2. **Reasonable sampling**: adjust sampling rates by metric importance
3. **Pre-aggregation**: precompute common dimensions to reduce query latency
4. **Incremental computation**: time-window-based incremental update mechanisms

---

> [**Note**: each game type should pick the metric subset that fits its business, avoiding the performance problems and analysis complexity of over-collection. Start from the foundation metrics and expand step by step toward advanced analytics.]
