package store

import "encoding/json"

// gorm row structs for the demo_* tables. These are demo-owned tables in a
// dedicated database (never the platform "croupier" DB — migrate.go guards
// that), so AutoMigrate is the schema source of truth here; the numbered
// goose chain applies to internal/model only.

type demoPlayerRow struct {
	ID          string `gorm:"primaryKey;column:id;size:64"`
	Nickname    string `gorm:"column:nickname;size:128"`
	Character   string `gorm:"column:character;size:128"`
	ServerID    string `gorm:"column:server_id;size:64"`
	Level       int    `gorm:"column:level"`
	VIP         int    `gorm:"column:vip"`
	Status      string `gorm:"column:status;size:32"`
	BalanceGold int64  `gorm:"column:balance_gold"`
	LastLoginAt string `gorm:"column:last_login_at;size:40"`
	CreatedAt   string `gorm:"column:created_at;size:40"`
}

func (demoPlayerRow) TableName() string { return "demo_players" }

func playerToRow(p Player) demoPlayerRow {
	return demoPlayerRow{
		ID: p.ID, Nickname: p.Nickname, Character: p.Character, ServerID: p.ServerID,
		Level: p.Level, VIP: p.VIP, Status: p.Status, BalanceGold: p.BalanceGold,
		LastLoginAt: p.LastLoginAt, CreatedAt: p.CreatedAt,
	}
}

func (r demoPlayerRow) record() Player {
	return Player{
		ID: r.ID, Nickname: r.Nickname, Character: r.Character, ServerID: r.ServerID,
		Level: r.Level, VIP: r.VIP, Status: r.Status, BalanceGold: r.BalanceGold,
		LastLoginAt: r.LastLoginAt, CreatedAt: r.CreatedAt,
	}
}

type demoTaskRow struct {
	ID              string  `gorm:"primaryKey;column:id;size:64"`
	Type            string  `gorm:"column:type;size:32"`
	Title           string  `gorm:"column:title;size:256"`
	Detail          string  `gorm:"column:detail;type:text"`
	ReceiverRange   string  `gorm:"column:receiver_range;size:32"`
	TargetServerID  *string `gorm:"column:target_server_id;size:64"`
	TargetPlayerID  *string `gorm:"column:target_player_id;size:64"`
	TargetPlayerIDs string  `gorm:"column:target_player_ids;type:text"`
	TargetOrderNo   *string `gorm:"column:target_order_no;size:64"`
	TargetCount     int     `gorm:"column:target_count"`
	PerAmount       *int64  `gorm:"column:per_amount"`
	TotalAmount     *int64  `gorm:"column:total_amount"`
	Currency        *string `gorm:"column:currency;size:16"`
	Status          string  `gorm:"column:status;size:32"`
	SentCount       int     `gorm:"column:sent_count"`
	FailedCount     int     `gorm:"column:failed_count"`
	Operator        string  `gorm:"column:operator;size:64"`
	ApprovalID      *string `gorm:"column:approval_id;size:64"`
	CreatedAt       string  `gorm:"column:created_at;size:40"`
}

func (demoTaskRow) TableName() string { return "demo_mail_tasks" }

func taskToRow(t Task) (demoTaskRow, error) {
	ids, err := json.Marshal(t.TargetPlayerIDs)
	if err != nil {
		return demoTaskRow{}, err
	}
	return demoTaskRow{
		ID: t.ID, Type: t.Type, Title: t.Title, Detail: t.Detail,
		ReceiverRange:  t.ReceiverRange,
		TargetServerID: t.TargetServerID, TargetPlayerID: t.TargetPlayerID,
		TargetPlayerIDs: string(ids), TargetOrderNo: t.TargetOrderNo,
		TargetCount: t.TargetCount, PerAmount: t.PerAmount, TotalAmount: t.TotalAmount,
		Currency: t.Currency, Status: t.Status, SentCount: t.SentCount,
		FailedCount: t.FailedCount, Operator: t.Operator, ApprovalID: t.ApprovalID,
		CreatedAt: t.CreatedAt,
	}, nil
}

func (r demoTaskRow) record() (Task, error) {
	var ids []string
	if r.TargetPlayerIDs != "" {
		if err := json.Unmarshal([]byte(r.TargetPlayerIDs), &ids); err != nil {
			return Task{}, err
		}
	}
	if ids == nil {
		ids = []string{}
	}
	return Task{
		ID: r.ID, Type: r.Type, Title: r.Title, Detail: r.Detail,
		ReceiverRange:  r.ReceiverRange,
		TargetServerID: r.TargetServerID, TargetPlayerID: r.TargetPlayerID,
		TargetPlayerIDs: ids, TargetOrderNo: r.TargetOrderNo,
		TargetCount: r.TargetCount, PerAmount: r.PerAmount, TotalAmount: r.TotalAmount,
		Currency: r.Currency, Status: r.Status, SentCount: r.SentCount,
		FailedCount: r.FailedCount, Operator: r.Operator, ApprovalID: r.ApprovalID,
		CreatedAt: r.CreatedAt,
	}, nil
}

type demoBanRow struct {
	ID            string `gorm:"primaryKey;column:id;size:64"`
	PlayerID      string `gorm:"column:player_id;size:64"`
	Nickname      string `gorm:"column:nickname;size:128"`
	ServerID      string `gorm:"column:server_id;size:64"`
	ApprovalID    string `gorm:"column:approval_id;size:64"`
	Reason        string `gorm:"column:reason;size:64"`
	DurationHours int    `gorm:"column:duration_hours"`
	BannedAt      string `gorm:"column:banned_at;size:40"`
	ExpiresAt     string `gorm:"column:expires_at;size:40"`
	Status        string `gorm:"column:status;size:32"`
	Operator      string `gorm:"column:operator;size:64"`
	Approver      string `gorm:"column:approver;size:64"`
}

func (demoBanRow) TableName() string { return "demo_bans" }

func banToRow(b Ban) demoBanRow {
	return demoBanRow{
		ID: b.ID, PlayerID: b.PlayerID, Nickname: b.Nickname, ServerID: b.ServerID,
		ApprovalID: b.ApprovalID, Reason: b.Reason, DurationHours: b.DurationHours,
		BannedAt: b.BannedAt, ExpiresAt: b.ExpiresAt, Status: b.Status,
		Operator: b.Operator, Approver: b.Approver,
	}
}

func (r demoBanRow) record() Ban {
	return Ban{
		ID: r.ID, PlayerID: r.PlayerID, Nickname: r.Nickname, ServerID: r.ServerID,
		ApprovalID: r.ApprovalID, Reason: r.Reason, DurationHours: r.DurationHours,
		BannedAt: r.BannedAt, ExpiresAt: r.ExpiresAt, Status: r.Status,
		Operator: r.Operator, Approver: r.Approver,
	}
}

type demoOrderRow struct {
	OrderNo        string  `gorm:"primaryKey;column:order_no;size:64"`
	PlayerID       string  `gorm:"column:player_id;size:64"`
	Nickname       string  `gorm:"column:nickname;size:128"`
	Amount         int64   `gorm:"column:amount"`
	Currency       string  `gorm:"column:currency;size:16"`
	Channel        string  `gorm:"column:channel;size:32"`
	Status         string  `gorm:"column:status;size:32"`
	GameCredit     string  `gorm:"column:game_credit;size:32"`
	Reconciliation string  `gorm:"column:reconciliation;size:32"`
	PayTransaction string  `gorm:"column:pay_transaction_id;size:128"`
	GameCreditRef  *string `gorm:"column:game_credit_ref;size:128"`
	PaidAt         string  `gorm:"column:paid_at;size:40"`
	CreditedAt     *string `gorm:"column:credited_at;size:40"`
}

func (demoOrderRow) TableName() string { return "demo_orders" }

func orderToRow(o Order) demoOrderRow {
	return demoOrderRow{
		OrderNo: o.OrderNo, PlayerID: o.PlayerID, Nickname: o.Nickname,
		Amount: o.Amount, Currency: o.Currency, Channel: o.Channel,
		Status: o.Status, GameCredit: o.GameCredit, Reconciliation: o.Reconciliation,
		PayTransaction: o.PayTransaction, GameCreditRef: o.GameCreditRef,
		PaidAt: o.PaidAt, CreditedAt: o.CreditedAt,
	}
}

func (r demoOrderRow) record() Order {
	return Order{
		OrderNo: r.OrderNo, PlayerID: r.PlayerID, Nickname: r.Nickname,
		Amount: r.Amount, Currency: r.Currency, Channel: r.Channel,
		Status: r.Status, GameCredit: r.GameCredit, Reconciliation: r.Reconciliation,
		PayTransaction: r.PayTransaction, GameCreditRef: r.GameCreditRef,
		PaidAt: r.PaidAt, CreditedAt: r.CreditedAt,
	}
}

type demoActivityRow struct {
	ID             string `gorm:"primaryKey;column:id;size:64"`
	Name           string `gorm:"column:name;size:256"`
	Type           string `gorm:"column:type;size:64"`
	Timezone       string `gorm:"column:timezone;size:64"`
	StartAt        string `gorm:"column:start_at;size:40"`
	EndAt          string `gorm:"column:end_at;size:40"`
	Status         string `gorm:"column:status;size:32"`
	CurrentVersion int    `gorm:"column:current_version"`
}

func (demoActivityRow) TableName() string { return "demo_activities" }

type demoActivityVersionRow struct {
	ActivityID  string `gorm:"primaryKey;column:activity_id;size:64"`
	Version     int    `gorm:"primaryKey;column:version"`
	Note        string `gorm:"column:note;size:256"`
	PublishedAt string `gorm:"column:published_at;size:40"`
	Operator    string `gorm:"column:operator;size:64"`
}

func (demoActivityVersionRow) TableName() string { return "demo_activity_versions" }

func activityToRows(a Activity) (demoActivityRow, []demoActivityVersionRow) {
	row := demoActivityRow{
		ID: a.ID, Name: a.Name, Type: a.Type, Timezone: a.Timezone,
		StartAt: a.StartAt, EndAt: a.EndAt, Status: a.Status,
		CurrentVersion: a.CurrentVersion,
	}
	versions := make([]demoActivityVersionRow, 0, len(a.Versions))
	for _, v := range a.Versions {
		versions = append(versions, demoActivityVersionRow{
			ActivityID: a.ID, Version: v.Version, Note: v.Note,
			PublishedAt: v.PublishedAt, Operator: v.Operator,
		})
	}
	return row, versions
}

func (r demoActivityRow) record(versions []demoActivityVersionRow) Activity {
	a := Activity{
		ID: r.ID, Name: r.Name, Type: r.Type, Timezone: r.Timezone,
		StartAt: r.StartAt, EndAt: r.EndAt, Status: r.Status,
		CurrentVersion: r.CurrentVersion,
		Versions:       []ActivityVersion{},
	}
	for _, v := range versions {
		a.Versions = append(a.Versions, ActivityVersion{
			Version: v.Version, Note: v.Note, PublishedAt: v.PublishedAt, Operator: v.Operator,
		})
	}
	return a
}

// demoAuditRow is the append-only audit table. Seq is an auto-increment rowid
// so reload ordering matches insertion (newest insert = newest op, matching
// the in-memory prepend order).
type demoAuditRow struct {
	Seq         uint64  `gorm:"primaryKey;column:seq"`
	OpID        string  `gorm:"column:op_id;size:64;uniqueIndex"`
	At          string  `gorm:"column:at;size:40"`
	Operator    string  `gorm:"column:operator;size:64"`
	Action      string  `gorm:"column:action;size:64"`
	Target      string  `gorm:"column:target;size:128"`
	Result      string  `gorm:"column:result;size:64"`
	OperationID string  `gorm:"column:operation_id;size:64"`
	ApprovalID  *string `gorm:"column:approval_id;size:64"`
}

func (demoAuditRow) TableName() string { return "demo_audit_log" }

func auditToRow(o OpRecord) demoAuditRow {
	return demoAuditRow{
		OpID: o.ID, At: o.At, Operator: o.Operator, Action: o.Action,
		Target: o.Target, Result: o.Result, OperationID: o.OperationID,
		ApprovalID: o.ApprovalID,
	}
}

func (r demoAuditRow) record() OpRecord {
	return OpRecord{
		ID: r.OpID, At: r.At, Operator: r.Operator, Action: r.Action,
		Target: r.Target, Result: r.Result, OperationID: r.OperationID,
		ApprovalID: r.ApprovalID,
	}
}

type demoIdemRow struct {
	Seq         uint64 `gorm:"primaryKey;column:seq"`
	GameID      string `gorm:"column:game_id;size:64;uniqueIndex:idx_demo_idem_scope"`
	Env         string `gorm:"column:env;size:32;uniqueIndex:idx_demo_idem_scope"`
	Op          string `gorm:"column:op;size:64;uniqueIndex:idx_demo_idem_scope"`
	OperationID string `gorm:"column:operation_id;size:128;uniqueIndex:idx_demo_idem_scope"`
	Fingerprint string `gorm:"column:fingerprint;size:80"`
	Status      string `gorm:"column:status;size:16"`
	Receipt     []byte `gorm:"column:receipt;type:text"`
}

func (demoIdemRow) TableName() string { return "demo_idempotency" }

type demoMetaRow struct {
	ID           int  `gorm:"primaryKey;column:id"`
	SeedN        int  `gorm:"column:seed_n"`
	Seeded       bool `gorm:"column:seeded"`
	ReportRunSeq int  `gorm:"column:report_run_seq"`
}

func (demoMetaRow) TableName() string { return "demo_meta" }

// allModels is the AutoMigrate list.
func allModels() []any {
	return []any{
		demoPlayerRow{}, demoTaskRow{}, demoBanRow{}, demoOrderRow{},
		demoActivityRow{}, demoActivityVersionRow{}, demoAuditRow{},
		demoIdemRow{}, demoMetaRow{},
	}
}
