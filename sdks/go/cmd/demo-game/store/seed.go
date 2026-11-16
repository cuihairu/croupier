package store

import (
	"errors"
	"fmt"
	"time"
)

// errIdemRowMissing is returned when a commit targets an idempotency row
// that is not armed (protocol misuse).
var errIdemRowMissing = errors.New("idempotency row missing")

// DefaultSeedAnchor is the fixed seed anchor date: seed timestamps are all
// derived from it (never wall-clock), so golden numbers recomputable per
// DATA-008 stay stable across runs and machines.
var DefaultSeedAnchor = time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)

// EnsureSeed seeds the deterministic demo dataset.
//
// Semantics:
//   - first run (never seeded): seed with N and commit the seed marker.
//   - same N again: no-op (the seed operation replays via the idempotency
//     protocol, like any other committed operation).
//   - different N without reseed: refused with an explicit error.
//   - reseed: wipe every demo table (including idempotency history and
//     audit) and seed fresh.
//
// The dataset structure (row counts, statuses, reconciliation markers,
// fixture IDs like task-mail-1002 / order 2026102308 / act-4002) is
// IDENTICAL for every N — executor scenarios depend on it. mulberry32(N)
// varies only cosmetic values (balances, order amounts, last-login
// offsets), so different seeds give visually different but structurally
// identical demo data.
func (s *Store) EnsureSeed(n int, anchorMS int64, reseed bool) (bool, error) {
	if n <= 0 {
		return false, fmt.Errorf("seed must be a positive integer")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.meta.Seeded && s.meta.SeedN != n && !reseed {
		return false, fmt.Errorf(
			"store already seeded with seed %d; refusing seed %d (pass -reseed to replace)",
			s.meta.SeedN, n)
	}
	if s.meta.Seeded && reseed {
		if err := s.backend.WipeAll(); err != nil {
			return false, fmt.Errorf("wipe before reseed: %w", err)
		}
	}
	key := IdemKey{GameID: s.gameID, Env: s.env, Op: "seed", OperationID: fmt.Sprintf("seed-%d", n)}
	res, err := s.backend.BeginOp(key, fmt.Sprintf("seed-%d", n))
	if err != nil {
		return false, err
	}
	switch res.Decision {
	case BeginReplay:
		return false, nil // same N already committed: no-op
	case BeginConflict:
		return false, errIdemRowMissing // unreachable: fingerprint == seed identity
	}

	snap := buildSeed(n, anchorMS)
	s.players = snap.players
	s.tasks = snap.tasks
	s.bans = snap.bans
	s.orders = snap.orders
	s.activities = snap.activities
	s.opRecords = snap.opRecords
	s.opSeq = snap.opSeq
	s.reportRunSeq = 0
	s.meta = MetaState{SeedN: n, Seeded: true, ReportRunSeq: 0}
	s.servers = append([]Server(nil), seedServers...)

	set := WriteSet{
		Players:    s.players,
		Tasks:      s.tasks,
		Bans:       s.bans,
		Orders:     s.orders,
		Activities: s.activities,
		Audit:      s.opRecords,
		Meta:       s.metaLocked(),
	}
	if err := s.backend.CommitOp(key, []byte(`{"seeded":true}`), set); err != nil {
		return false, fmt.Errorf("commit seed: %w", err)
	}
	return true, nil
}

// mulberry32 is a fixed-seed PRNG (tiny, deterministic, no deps). Same N →
// the exact same value stream.
func mulberry32(seed uint32) func() uint32 {
	a := seed
	return func() uint32 {
		a += 0x6D2B79F5
		t := a
		t = (t ^ (t >> 15)) * (t | 1)
		t ^= t + (t^(t>>7))*(t|61)
		return t ^ (t >> 14)
	}
}

func randBelow(rng func() uint32, n int) int64 {
	if n <= 0 {
		return 0
	}
	return int64(rng() % uint32(n))
}

// buildSeed ports the gameops-demo resetLocked seed with two changes:
// timestamps derive from the fixed anchor, and mulberry32(N) drives the
// cosmetic value jitter (see EnsureSeed).
func buildSeed(n int, anchorMS int64) snapshot {
	rng := mulberry32(uint32(n))
	iso := func(msAgo int64) string { return millisISO(anchorMS - msAgo) }

	players := make([]Player, 0, 26)
	for i := 0; i < 26; i++ {
		seq := i + 1
		name := "玩家" + intStr(seq) + "号"
		if seq == 13 {
			name = longNickname
		}
		players = append(players, Player{
			ID:          "200" + padLeft(intStr(seq), 2, '0'),
			Nickname:    name,
			Character:   "角色" + intStr(seq),
			ServerID:    seedServers[seq%len(seedServers)].ID,
			Level:       5 + ((seq * 7) % 55),
			VIP:         seq % 7,
			Status:      ternary(seq%7 == 0, "banned", "active"),
			BalanceGold: randBelow(rng, 9000),
			LastLoginAt: iso(int64(seq)*3_600_000 + randBelow(rng, 2_700_000)),
			CreatedAt:   iso(int64(seq) * 86_400_000 * 3),
		})
	}

	tasks := []Task{
		{
			ID: "task-mail-1001", Type: "mail", Title: "S3 赛季结算奖励",
			Detail:        "恭喜完成 S3 赛季，奖励已随信发放。",
			ReceiverRange: "all", TargetPlayerIDs: []string{}, TargetCount: 26,
			Status: "sent", SentCount: 26, Operator: "gm01",
			CreatedAt: iso(49 * 3_600_000),
		},
		{
			ID: "task-mail-1002", Type: "mail", Title: "停机维护补偿",
			Detail:        "感谢理解，维护补偿见附件。",
			ReceiverRange: "server", TargetServerID: strPtr("s1"), TargetPlayerIDs: []string{}, TargetCount: 9,
			Status: "partial_failed", SentCount: 7, FailedCount: 2, Operator: "operator",
			CreatedAt: iso(20 * 3_600_000),
		},
		{
			ID: "task-comp-2001", Type: "compensation", Title: "回档补偿",
			Detail:        "10-21 回档事故全员补偿",
			ReceiverRange: "all", TargetPlayerIDs: []string{}, TargetCount: 26,
			PerAmount: int64Ptr(500), TotalAmount: int64Ptr(13000), Currency: strPtr("金"),
			Status: "pending_approval", Operator: "operator", ApprovalID: strPtr("approval-comp-2001"),
			CreatedAt: iso(2 * 3_600_000),
		},
		{
			ID: "task-comp-2002", Type: "compensation", Title: "充值未到账补偿",
			Detail:        "订单 20261023001 补发",
			ReceiverRange: "players", TargetPlayerID: strPtr("20008"), TargetPlayerIDs: []string{"20008"}, TargetOrderNo: strPtr("2026102308"),
			TargetCount: 1, PerAmount: int64Ptr(648), TotalAmount: int64Ptr(648), Currency: strPtr("金"),
			Status: "completed", SentCount: 1, Operator: "gm01", ApprovalID: strPtr("approval-comp-2002"),
			CreatedAt: iso(30 * 3_600_000),
		},
	}

	reasons := []string{"cheating", "fraud", "abuse", "risk"}
	bans := make([]Ban, 0, 12)
	for i := 0; i < 12; i++ {
		seq := i + 1
		player := players[(seq*3)%len(players)]
		duration := []int{24, 72, 168, 720}[seq%4]
		status := "active"
		if seq%3 == 0 {
			status = "expired"
		} else if seq%5 == 0 {
			status = "lifted"
		}
		bans = append(bans, Ban{
			ID:       "ban-300" + padLeft(intStr(seq), 2, '0'),
			PlayerID: player.ID, Nickname: player.Nickname, ServerID: player.ServerID,
			ApprovalID: "approval-ban-" + padLeft(intStr(seq), 4, '0'),
			Reason:     reasons[seq%len(reasons)], DurationHours: duration,
			BannedAt:  iso(int64(seq+2) * 86_400_000),
			ExpiresAt: iso(int64(seq+2)*86_400_000 - int64(duration)*3_600_000),
			Status:    status,
			Operator:  ternary(seq%2 == 0, "operator", "gm01"), Approver: "admin",
		})
	}

	channels := []string{"official", "ios", "android", "wechat", "alipay"}
	amounts := []int64{6, 30, 98, 328, 648}
	orders := make([]Order, 0, 30)
	for i := 0; i < 30; i++ {
		seq := i + 1
		player := players[(seq*5)%len(players)]
		missingCredit := seq == 8 || seq == 17
		missingOrder := seq == 24
		failed := seq%11 == 0
		gameCredit := "credited"
		if failed {
			gameCredit = "not_needed"
		} else if missingCredit {
			gameCredit = "missing"
		} else if seq == 26 {
			gameCredit = "pending"
		}
		status := "credited"
		if failed {
			status = "failed"
		} else if missingCredit || seq == 26 {
			status = "paid"
		}
		reconciliation := "match"
		if missingCredit {
			reconciliation = "missing_credit"
		} else if missingOrder {
			reconciliation = "missing_order"
		}
		var ref *string
		var creditedAt *string
		if gameCredit == "credited" {
			ref = strPtr("credit-" + intStr(7000+seq))
			creditedAt = strPtr(iso(int64(seq)*5*3_600_000 - 60_000))
		}
		orders = append(orders, Order{
			OrderNo: "2026102" + intStr(300+seq), PlayerID: player.ID, Nickname: player.Nickname,
			Amount:   amounts[randBelow(rng, len(amounts))],
			Currency: ternary(seq%9 == 0, "USD", "CNY"), Channel: channels[seq%len(channels)],
			Status: status, GameCredit: gameCredit, Reconciliation: reconciliation,
			PayTransaction: "pay-" + intStr(9000+seq), GameCreditRef: ref,
			PaidAt: iso(int64(seq) * 5 * 3_600_000), CreditedAt: creditedAt,
		})
	}

	activityTypes := []string{"login_gift", "rank_reward", "shop_discount", "holiday_event"}
	timezones := []string{"Asia/Shanghai", "UTC", "America/Los_Angeles"}
	activities := make([]Activity, 0, 8)
	for i := 0; i < 8; i++ {
		seq := i + 1
		versionCount := (seq % 3) + 1
		versions := make([]ActivityVersion, 0, versionCount)
		for v := 0; v < versionCount; v++ {
			note := "初版配置"
			if v > 0 {
				note = "调整奖励数值（v" + intStr(v+1) + "）"
			}
			versions = append(versions, ActivityVersion{
				Version: v + 1, Note: note,
				PublishedAt: iso(int64(versionCount-v) * 86_400_000),
				Operator:    ternary(v%2 == 0, "gm01", "operator"),
			})
		}
		status := "ended"
		switch seq % 4 {
		case 0:
			status = "active"
		case 1:
			status = "configured"
		case 2:
			status = "published"
		}
		activities = append(activities, Activity{
			ID:   "act-400" + intStr(seq),
			Name: []string{"每日登录礼", "巅峰排行榜", "限时折扣商店", "节日庆典"}[seq%4] + "·第" + intStr(seq) + "期",
			Type: activityTypes[seq%len(activityTypes)], Timezone: timezones[seq%len(timezones)],
			StartAt: iso(-int64(seq) * 7 * 86_400_000), EndAt: iso(int64(seq) * 86_400_000),
			Status: status, CurrentVersion: versionCount, Versions: versions,
		})
	}

	opRecords := []OpRecord{
		{ID: "op-5001", At: iso(26 * 3_600_000), Operator: "operator", Action: "player.ban",
			Target: "ban-30003", Result: "success", OperationID: "op-5001", ApprovalID: strPtr("approval-ban-0003")},
		{ID: "op-5002", At: iso(2 * 3_600_000), Operator: "operator", Action: "compensation.grant",
			Target: "task-comp-2001", Result: "pending_approval", OperationID: "op-5002", ApprovalID: strPtr("approval-comp-2001")},
		{ID: "op-5003", At: iso(8 * 3_600_000), Operator: "gm01", Action: "activity.publish",
			Target: "act-4003", Result: "success", OperationID: "op-5003"},
	}

	return snapshot{
		players:    players,
		tasks:      tasks,
		bans:       bans,
		orders:     orders,
		activities: activities,
		opRecords:  opRecords,
		opSeq:      len(opRecords),
	}
}

// longNickname is the E1 fixture: the over-length nickname that exercises
// text truncation/ellipsis in list UIs (player seq 13).
var longNickname = "风起云涌一区无敌战神丶樱花漫天飞舞的超级无敌可爱小龙女本尊在此"
