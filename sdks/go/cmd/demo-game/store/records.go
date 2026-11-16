// Package store implements the demo-game persistent game state.
//
// Package layout: records.go holds the plain domain records (identical JSON
// shapes to examples/gameops-demo), store.go holds the in-memory Store with
// transactional write-through, backend.go/sql.go/memory.go hold the durable
// backends, idempotency.go the exactly-once protocol, seed.go the
// deterministic seeding, migrate.go the DSN/driver switch.
//
// The demo game is a SIMULATED integrator: it exercises the platform side of
// the game-integration acceptance flows. It is not a real game server.
package store

// Server is a static game-server shard. Servers have no mutation contract;
// the trio is re-seeded on every load (documented boundary).
type Server struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"`
}

// Player mirrors gameOpsPlayer (JSON field names must stay aligned with the
// E1 web seed and the playerSchema contract).
type Player struct {
	ID          string `json:"id"`
	Nickname    string `json:"nickname"`
	Character   string `json:"character"`
	ServerID    string `json:"serverId"`
	Level       int    `json:"level"`
	VIP         int    `json:"vip"`
	Status      string `json:"status"`
	BalanceGold int64  `json:"balanceGold"`
	LastLoginAt string `json:"lastLoginAt"`
	CreatedAt   string `json:"createdAt"`
}

// Task mirrors gameOpsTask (mail_task.* contract).
type Task struct {
	ID              string   `json:"id"`
	Type            string   `json:"type"`
	Title           string   `json:"title"`
	Detail          string   `json:"detail"`
	ReceiverRange   string   `json:"receiverRange"`
	TargetServerID  *string  `json:"targetServerId"`
	TargetPlayerID  *string  `json:"targetPlayerId"`
	TargetPlayerIDs []string `json:"targetPlayerIds"`
	TargetOrderNo   *string  `json:"targetOrderNo"`
	TargetCount     int      `json:"targetCount"`
	PerAmount       *int64   `json:"perAmount"`
	TotalAmount     *int64   `json:"totalAmount"`
	Currency        *string  `json:"currency"`
	Status          string   `json:"status"`
	SentCount       int      `json:"sentCount"`
	FailedCount     int      `json:"failedCount"`
	Operator        string   `json:"operator"`
	ApprovalID      *string  `json:"approvalId"`
	CreatedAt       string   `json:"createdAt"`
}

// Ban mirrors gameOpsBan (ban.* contract).
type Ban struct {
	ID            string `json:"id"`
	PlayerID      string `json:"playerId"`
	Nickname      string `json:"nickname"`
	ServerID      string `json:"serverId"`
	ApprovalID    string `json:"approvalId"`
	Reason        string `json:"reason"`
	DurationHours int    `json:"durationHours"`
	BannedAt      string `json:"bannedAt"`
	ExpiresAt     string `json:"expiresAt"`
	Status        string `json:"status"`
	Operator      string `json:"operator"`
	Approver      string `json:"approver"`
}

// Order mirrors gameOpsOrder (order.* contract; keyed by OrderNo).
type Order struct {
	OrderNo        string  `json:"orderNo"`
	PlayerID       string  `json:"playerId"`
	Nickname       string  `json:"nickname"`
	Amount         int64   `json:"amount"`
	Currency       string  `json:"currency"`
	Channel        string  `json:"channel"`
	Status         string  `json:"status"`
	GameCredit     string  `json:"gameCredit"`
	Reconciliation string  `json:"reconciliation"`
	PayTransaction string  `json:"payTransactionId"`
	GameCreditRef  *string `json:"gameCreditRef"`
	PaidAt         string  `json:"paidAt"`
	CreditedAt     *string `json:"creditedAt"`
}

// ActivityVersion is one published config revision of an activity. Version
// rows are kept forever (rollback audit), UNIQUE per (activity, version).
type ActivityVersion struct {
	Version     int    `json:"version"`
	Note        string `json:"note"`
	PublishedAt string `json:"publishedAt"`
	Operator    string `json:"operator"`
}

// Activity mirrors gameOpsActivity (activity.* contract).
type Activity struct {
	ID             string            `json:"id"`
	Name           string            `json:"name"`
	Type           string            `json:"type"`
	Timezone       string            `json:"timezone"`
	StartAt        string            `json:"startAt"`
	EndAt          string            `json:"endAt"`
	Status         string            `json:"status"`
	CurrentVersion int               `json:"currentVersion"`
	Versions       []ActivityVersion `json:"versions"`
}

// OpRecord is one append-only audit entry (demo_audit_log).
type OpRecord struct {
	ID          string  `json:"id"`
	At          string  `json:"at"`
	Operator    string  `json:"operator"`
	Action      string  `json:"action"`
	Target      string  `json:"target"`
	Result      string  `json:"result"`
	OperationID string  `json:"operationId"`
	ApprovalID  *string `json:"approvalId"`
}

// MetaState is the single demo_meta row: seed marker + report run counter.
type MetaState struct {
	SeedN        int  `json:"seedN"`
	ReportRunSeq int  `json:"reportRunSeq"`
	Seeded       bool `json:"seeded"`
}

// snapshot is a deep copy of all mutable state, taken at Begin (rollback
// source) and diffed against current state at Commit (delete detection).
type snapshot struct {
	players    []Player
	tasks      []Task
	bans       []Ban
	orders     []Order
	activities []Activity
	opRecords  []OpRecord
	opSeq      int
}

// WriteSet is the unit a backend applies atomically: every table is written
// as "upsert all current rows, delete rows missing from the current set"
// (the snapshot diff supplies delete detection); audit rows are
// insert-missing (append-only); meta is upserted.
type WriteSet struct {
	Players    []Player
	Tasks      []Task
	Bans       []Ban
	Orders     []Order
	Activities []Activity
	Audit      []OpRecord
	Meta       MetaState
}
