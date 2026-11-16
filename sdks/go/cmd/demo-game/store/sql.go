package store

import (
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// sqlBackend persists demo state through gorm (postgres or pure-Go sqlite,
// selected in migrate.go). Every CommitOp runs in one transaction: business
// rows + audit + idempotency receipt commit together, so a crash leaves
// either the pre-operation state plus a releasable pending row, or the
// committed state with its replayable receipt.
type sqlBackend struct {
	db *gorm.DB
}

func (b *sqlBackend) LoadAll() ([]Player, []Task, []Ban, []Order, []Activity, []OpRecord, MetaState, error) {
	var playerRows []demoPlayerRow
	if err := b.db.Find(&playerRows).Error; err != nil {
		return nil, nil, nil, nil, nil, nil, MetaState{}, err
	}
	players := make([]Player, 0, len(playerRows))
	for _, r := range playerRows {
		players = append(players, r.record())
	}

	var taskRows []demoTaskRow
	if err := b.db.Find(&taskRows).Error; err != nil {
		return nil, nil, nil, nil, nil, nil, MetaState{}, err
	}
	tasks := make([]Task, 0, len(taskRows))
	for _, r := range taskRows {
		rec, err := r.record()
		if err != nil {
			return nil, nil, nil, nil, nil, nil, MetaState{}, err
		}
		tasks = append(tasks, rec)
	}

	var banRows []demoBanRow
	if err := b.db.Find(&banRows).Error; err != nil {
		return nil, nil, nil, nil, nil, nil, MetaState{}, err
	}
	bans := make([]Ban, 0, len(banRows))
	for _, r := range banRows {
		bans = append(bans, r.record())
	}

	var orderRows []demoOrderRow
	if err := b.db.Find(&orderRows).Error; err != nil {
		return nil, nil, nil, nil, nil, nil, MetaState{}, err
	}
	orders := make([]Order, 0, len(orderRows))
	for _, r := range orderRows {
		orders = append(orders, r.record())
	}

	var activityRows []demoActivityRow
	if err := b.db.Find(&activityRows).Error; err != nil {
		return nil, nil, nil, nil, nil, nil, MetaState{}, err
	}
	var versionRows []demoActivityVersionRow
	if err := b.db.Order("activity_id, version").Find(&versionRows).Error; err != nil {
		return nil, nil, nil, nil, nil, nil, MetaState{}, err
	}
	versionsByActivity := map[string][]demoActivityVersionRow{}
	for _, r := range versionRows {
		versionsByActivity[r.ActivityID] = append(versionsByActivity[r.ActivityID], r)
	}
	activities := make([]Activity, 0, len(activityRows))
	for _, r := range activityRows {
		activities = append(activities, r.record(versionsByActivity[r.ID]))
	}

	var auditRows []demoAuditRow
	if err := b.db.Order("seq").Find(&auditRows).Error; err != nil {
		return nil, nil, nil, nil, nil, nil, MetaState{}, err
	}
	audit := make([]OpRecord, 0, len(auditRows))
	for _, r := range auditRows {
		audit = append(audit, r.record())
	}

	var metas []demoMetaRow
	// Find (not First) so an empty meta table on first boot is not logged as
	// a red "record not found" error.
	if err := b.db.Where("id = ?", 1).Limit(1).Find(&metas).Error; err != nil {
		return nil, nil, nil, nil, nil, nil, MetaState{}, err
	}
	meta := demoMetaRow{ID: 1}
	if len(metas) > 0 {
		meta = metas[0]
	}
	return players, tasks, bans, orders, activities, audit,
		MetaState{SeedN: meta.SeedN, Seeded: meta.Seeded, ReportRunSeq: meta.ReportRunSeq}, nil
}

func (b *sqlBackend) BeginOp(key IdemKey, fingerprint string) (BeginResult, error) {
	row := demoIdemRow{
		GameID: key.GameID, Env: key.Env, Op: key.Op, OperationID: key.OperationID,
		Fingerprint: fingerprint, Status: "pending",
	}
	err := b.db.Create(&row).Error
	if err == nil {
		return BeginResult{Decision: BeginProceed}, nil
	}
	// Insert failed: an earlier attempt left a row. Arbitrate by its state.
	var existing demoIdemRow
	qerr := b.db.First(&existing, "game_id = ? AND env = ? AND op = ? AND operation_id = ?",
		key.GameID, key.Env, key.Op, key.OperationID).Error
	if qerr != nil {
		// Neither insert nor read succeeded — a real backend error.
		return BeginResult{}, fmt.Errorf("begin operation: insert: %v; probe: %w", err, qerr)
	}
	switch {
	case existing.Fingerprint != fingerprint:
		return BeginResult{Decision: BeginConflict}, nil
	case existing.Status == "committed":
		return BeginResult{Decision: BeginReplay, Receipt: existing.Receipt}, nil
	default:
		// Stale pending row: the prior attempt crashed before commit (the
		// store never leaves pending rows on its error path). Release and
		// proceed; true result-unknown convergence stays with GAME-010.
		if rerr := b.ReleaseOp(key); rerr != nil {
			return BeginResult{}, fmt.Errorf("release stale pending operation: %w", rerr)
		}
		fresh := demoIdemRow{
			GameID: key.GameID, Env: key.Env, Op: key.Op, OperationID: key.OperationID,
			Fingerprint: fingerprint, Status: "pending",
		}
		if cerr := b.db.Create(&fresh).Error; cerr != nil {
			return BeginResult{}, fmt.Errorf("begin operation after release: %w", cerr)
		}
		return BeginResult{Decision: BeginProceed}, nil
	}
}

func (b *sqlBackend) CommitOp(key IdemKey, receipt []byte, set WriteSet) error {
	return b.db.Transaction(func(tx *gorm.DB) error {
		playerRows := make([]demoPlayerRow, 0, len(set.Players))
		for _, p := range set.Players {
			playerRows = append(playerRows, playerToRow(p))
		}
		if err := upsertAndPrune(tx, playerRows, "id", func(r demoPlayerRow) string { return r.ID }); err != nil {
			return err
		}

		taskRows := make([]demoTaskRow, 0, len(set.Tasks))
		for _, t := range set.Tasks {
			row, err := taskToRow(t)
			if err != nil {
				return err
			}
			taskRows = append(taskRows, row)
		}
		if err := upsertAndPrune(tx, taskRows, "id", func(r demoTaskRow) string { return r.ID }); err != nil {
			return err
		}

		banRows := make([]demoBanRow, 0, len(set.Bans))
		for _, bn := range set.Bans {
			banRows = append(banRows, banToRow(bn))
		}
		if err := upsertAndPrune(tx, banRows, "id", func(r demoBanRow) string { return r.ID }); err != nil {
			return err
		}

		orderRows := make([]demoOrderRow, 0, len(set.Orders))
		for _, o := range set.Orders {
			orderRows = append(orderRows, orderToRow(o))
		}
		if err := upsertAndPrune(tx, orderRows, "order_no", func(r demoOrderRow) string { return r.OrderNo }); err != nil {
			return err
		}

		activityRows := make([]demoActivityRow, 0, len(set.Activities))
		allVersions := make([]demoActivityVersionRow, 0, len(set.Activities)*2)
		for _, a := range set.Activities {
			row, versions := activityToRows(a)
			activityRows = append(activityRows, row)
			allVersions = append(allVersions, versions...)
		}
		if err := upsertAndPrune(tx, activityRows, "id", func(r demoActivityRow) string { return r.ID }); err != nil {
			return err
		}
		// Versions are few (seed ≈ 20 rows): rewrite the table each commit —
		// portable across drivers and keeps the UNIQUE(activity, version)
		// bookkeeping trivial.
		if err := tx.Exec(fmt.Sprintf("DELETE FROM %s", demoActivityVersionRow{}.TableName())).Error; err != nil {
			return err
		}
		if err := upsertAll(tx, allVersions); err != nil {
			return err
		}

		auditRows := make([]demoAuditRow, 0, len(set.Audit))
		for _, o := range set.Audit {
			auditRows = append(auditRows, auditToRow(o))
		}
		if len(auditRows) > 0 {
			if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&auditRows).Error; err != nil {
				return err
			}
		}

		if err := tx.Clauses(clause.OnConflict{UpdateAll: true}).Create(&demoMetaRow{
			ID: 1, SeedN: set.Meta.SeedN, Seeded: set.Meta.Seeded, ReportRunSeq: set.Meta.ReportRunSeq,
		}).Error; err != nil {
			return err
		}

		return tx.Model(&demoIdemRow{}).
			Where("game_id = ? AND env = ? AND op = ? AND operation_id = ?", key.GameID, key.Env, key.Op, key.OperationID).
			Updates(map[string]any{"status": "committed", "receipt": receipt}).Error
	})
}

func (b *sqlBackend) ReleaseOp(key IdemKey) error {
	return b.db.Where(
		"game_id = ? AND env = ? AND op = ? AND operation_id = ? AND status = ?",
		key.GameID, key.Env, key.Op, key.OperationID, "pending",
	).Delete(&demoIdemRow{}).Error
}

func (b *sqlBackend) UpdateMeta(meta MetaState) error {
	return b.db.Clauses(clause.OnConflict{UpdateAll: true}).Create(&demoMetaRow{
		ID: 1, SeedN: meta.SeedN, Seeded: meta.Seeded, ReportRunSeq: meta.ReportRunSeq,
	}).Error
}

func (b *sqlBackend) Close() error {
	database, err := b.db.DB()
	if err != nil {
		return err
	}
	return database.Close()
}

// WipeAll clears every demo table for a reseed.
func (b *sqlBackend) WipeAll() error {
	return b.db.Transaction(func(tx *gorm.DB) error {
		for _, model := range []any{
			&demoPlayerRow{}, &demoTaskRow{}, &demoBanRow{}, &demoOrderRow{},
			&demoActivityRow{}, &demoActivityVersionRow{}, &demoAuditRow{},
			&demoIdemRow{}, &demoMetaRow{},
		} {
			if err := tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(model).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// upsertAll inserts or updates every row; empty input is a no-op (gorm
// rejects empty-slice creates).
func upsertAll[T any](tx *gorm.DB, rows []T) error {
	if len(rows) == 0 {
		return nil
	}
	return tx.Clauses(clause.OnConflict{UpdateAll: true}).Create(&rows).Error
}

// upsertAndPrune writes one table as "upsert all current rows, delete rows
// missing from the current set" — the snapshot diff supplies delete
// detection without an explicit delete list. An empty set clears the table
// (scoped session; gorm blocks global deletes otherwise).
func upsertAndPrune[T any](tx *gorm.DB, rows []T, keyCol string, keyOf func(T) string) error {
	if len(rows) == 0 {
		return tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(new(T)).Error
	}
	if err := upsertAll(tx, rows); err != nil {
		return err
	}
	keep := make([]string, 0, len(rows))
	for i := range rows {
		keep = append(keep, keyOf(rows[i]))
	}
	return tx.Where(keyCol+" NOT IN ?", keep).Delete(new(T)).Error
}

// compile-time interface check
var _ Backend = (*sqlBackend)(nil)
