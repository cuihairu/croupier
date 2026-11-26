package worker

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	redis "github.com/redis/go-redis/v9"
)

// ---------------------------------------------------------------------------
// DATA-023: consumer group initialization policy.
//
//   - A new group starts at the declared position ("0" default = consume the
//     retained backlog; WORKER_GROUP_START=$ = latest only).
//   - An existing group is never repositioned at startup: its Redis-native
//     cursor keeps the consumption position across restarts, and a stale
//     application checkpoint cannot replay ACKed data.
//   - The persisted checkpoint is used only when creating a group (resume
//     after Redis lost the group) or through the explicit
//     WORKER_GROUP_RESET=true operator reset.
//   - Initialization failures are recorded, not silently discarded.
// ---------------------------------------------------------------------------

// seedEvent adds one valid event to the events stream and returns its id.
func seedEvent(t *testing.T, w *Worker, user string) string {
	t.Helper()
	data, err := json.Marshal(map[string]any{
		"ts": "2026-01-01T10:00:00Z", "game_id": "g1", "env": "prod",
		"user_id": user, "event": "login",
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	id, err := w.rdb.XAdd(context.Background(), &redis.XAddArgs{
		Stream: w.streamEvents,
		Values: map[string]any{"data": string(data)},
	}).Result()
	if err != nil {
		t.Fatalf("xadd: %v", err)
	}
	return id
}

// readNew delivers the group's never-delivered messages for a stream. Block
// is set explicitly: miniredis blocks forever on an empty XREADGROUP when no
// BLOCK argument is present (real Redis returns nil immediately).
func readNew(t *testing.T, w *Worker, stream string) []string {
	t.Helper()
	res, err := w.rdb.XReadGroup(context.Background(), &redis.XReadGroupArgs{
		Group:    w.group,
		Consumer: w.consumer,
		Streams:  []string{stream, ">"},
		Count:    100,
		Block:    10 * time.Millisecond,
	}).Result()
	if err == redis.Nil {
		return nil
	}
	if err != nil {
		t.Fatalf("xreadgroup: %v", err)
	}
	var ids []string
	for _, str := range res {
		for _, m := range str.Messages {
			ids = append(ids, m.ID)
		}
	}
	return ids
}

// groupCursor returns the group's last-delivered id via XINFO GROUPS.
func groupCursor(t *testing.T, w *Worker, stream string) string {
	t.Helper()
	groups, err := w.rdb.XInfoGroups(context.Background(), stream).Result()
	if err != nil {
		t.Fatalf("xinfogroups: %v", err)
	}
	for _, g := range groups {
		if g.Name == w.group {
			return g.LastDeliveredID
		}
	}
	t.Fatalf("group %q not found on %s", w.group, stream)
	return ""
}

// setidRecorderHook records XGROUP SETID commands passing through the client
// (implementing the full redis.Hook interface with pass-throughs).
type setidRecorderHook struct {
	ids []string
}

func (h *setidRecorderHook) DialHook(next redis.DialHook) redis.DialHook { return next }

func (h *setidRecorderHook) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return next
}

func (h *setidRecorderHook) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error {
		if cmd.Name() == "xgroup" && len(cmd.Args()) > 1 && cmd.Args()[1] == "setid" && len(cmd.Args()) > 4 {
			h.ids = append(h.ids, cmd.Args()[4].(string))
		}
		return next(ctx, cmd)
	}
}

// failGroupInitHook fails XINFO GROUPS and XGROUP CREATE so ensureGroups
// exercises both failure branches without a panic.
type failGroupInitHook struct{}

func (failGroupInitHook) DialHook(next redis.DialHook) redis.DialHook { return next }

func (failGroupInitHook) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return next
}

func (failGroupInitHook) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error {
		if cmd.Name() == "xinfo" || cmd.Name() == "xgroup" {
			return errors.New("forced group init failure")
		}
		return next(ctx, cmd)
	}
}

func TestGroupInit_LateWorkerConsumesBacklog(t *testing.T) {
	w, _ := testWorker(t)
	ctx := context.Background()
	// Events arrive BEFORE the worker/group ever starts.
	id1 := seedEvent(t, w, "u1")
	id2 := seedEvent(t, w, "u2")
	id3 := seedEvent(t, w, "u3")

	w.ensureGroups(ctx)

	got := readNew(t, w, w.streamEvents)
	if len(got) != 3 || got[0] != id1 || got[1] != id2 || got[2] != id3 {
		t.Fatalf("late-starting worker must consume the retained backlog, got %v", got)
	}
}

func TestGroupInit_LatestOnlyOptInSkipsBacklog(t *testing.T) {
	w, _ := testWorker(t)
	w.groupStart = "$"
	ctx := context.Background()
	seedEvent(t, w, "u1")
	seedEvent(t, w, "u2")

	w.ensureGroups(ctx)
	if got := readNew(t, w, w.streamEvents); len(got) != 0 {
		t.Fatalf("start=$ must skip the pre-existing backlog, got %v", got)
	}
	// Newly arriving events are still delivered.
	id3 := seedEvent(t, w, "u3")
	if got := readNew(t, w, w.streamEvents); len(got) != 1 || got[0] != id3 {
		t.Fatalf("start=$ must deliver new events, got %v", got)
	}
}

func TestParseGroupStart(t *testing.T) {
	cases := map[string]string{
		"":     "0",
		"0":    "0",
		" 0 ":  "0",
		"$":    "$",
		" $":   "$",
		"junk": "0",
		"0-1":  "0",
	}
	for in, want := range cases {
		if got := parseGroupStart(in); got != want {
			t.Errorf("parseGroupStart(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestGroupInit_ExistingGroupKeepsPositionDespiteStaleCheckpoint(t *testing.T) {
	w, mr := testWorker(t)
	ctx := context.Background()
	id1 := seedEvent(t, w, "u1")
	id2 := seedEvent(t, w, "u2")

	// First life: group created, both entries delivered and ACKed; the
	// checkpoint lands at id2.
	w.ensureGroups(ctx)
	if got := readNew(t, w, w.streamEvents); len(got) != 2 {
		t.Fatalf("first life should deliver both entries, got %v", got)
	}
	for _, id := range []string{id1, id2} {
		if err := w.rdb.XAck(ctx, w.streamEvents, w.group, id).Err(); err != nil {
			t.Fatalf("xack: %v", err)
		}
		w.advanceCheckpoint(ctx, w.streamEvents, id)
	}
	if got := w.rdb.Get(ctx, w.checkpointPrefix+":"+w.streamEvents).Val(); got != id2 {
		t.Fatalf("checkpoint = %q, want %q", got, id2)
	}

	// A stale/rolled-back application checkpoint points behind the group
	// cursor — exactly what must NOT rewind the group on restart.
	w.rdb.Set(ctx, w.checkpointPrefix+":"+w.streamEvents, id1, 0)

	// Second life: fresh worker on the same Redis.
	w2 := testWorkerOn(t, mr)
	rec := &setidRecorderHook{}
	w2.rdb.AddHook(rec)
	w2.ensureGroups(ctx)
	if len(rec.ids) != 0 {
		t.Fatalf("startup must never reposition an existing group, saw setid %v", rec.ids)
	}
	if cur := groupCursor(t, w2, w.streamEvents); cur != id2 {
		t.Fatalf("group cursor regressed to %q, want %q", cur, id2)
	}

	// Consumption continues where it stopped: no replay of id1/id2.
	id3 := seedEvent(t, w, "u3")
	if got := readNew(t, w2, w2.streamEvents); len(got) != 1 || got[0] != id3 {
		t.Fatalf("restart must keep the consumption position, got %v", got)
	}
}

func TestGroupInit_NewGroupAdoptsCheckpointAsStart(t *testing.T) {
	w, mr := testWorker(t)
	ctx := context.Background()
	seedEvent(t, w, "u1")
	id2 := seedEvent(t, w, "u2")
	id3 := seedEvent(t, w, "u3")

	// Redis lost the group (flush/replacement) but the checkpoint key
	// survives: a freshly created group resumes AT the checkpoint.
	w.rdb.Set(ctx, w.checkpointPrefix+":"+w.streamEvents, id2, 0)

	w2 := testWorkerOn(t, mr)
	w2.ensureGroups(ctx)
	if cur := groupCursor(t, w2, w.streamEvents); cur != id2 {
		t.Fatalf("new group should be created at the checkpoint %q, cursor %q", id2, cur)
	}
	if got := readNew(t, w2, w2.streamEvents); len(got) != 1 || got[0] != id3 {
		t.Fatalf("resume must deliver only post-checkpoint entries, got %v", got)
	}
}

func TestGroupInit_ExplicitResetRewindsGroup(t *testing.T) {
	w, mr := testWorker(t)
	ctx := context.Background()
	id1 := seedEvent(t, w, "u1")
	seedEvent(t, w, "u2")

	w.ensureGroups(ctx)
	readNew(t, w, w.streamEvents) // cursor now at id2

	// The only sanctioned reposition: WORKER_GROUP_RESET=true rewinds the
	// existing group to the persisted checkpoint (id1 here); the payments
	// stream has no checkpoint and falls back to the declared start "0".
	w.rdb.Set(ctx, w.checkpointPrefix+":"+w.streamEvents, id1, 0)
	w2 := testWorkerOn(t, mr)
	w2.groupReset = true
	rec := &setidRecorderHook{}
	w2.rdb.AddHook(rec)
	w2.ensureGroups(ctx)
	if len(rec.ids) != 2 || rec.ids[0] != id1 || rec.ids[1] != "0" {
		t.Fatalf("explicit reset must SETID events to %q and payments to \"0\", saw %v", id1, rec.ids)
	}
}

func TestGroupInit_CreateFailureIsRecordedNotSilent(t *testing.T) {
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	defer slog.SetDefault(prev)

	w, _ := testWorker(t)
	w.rdb.AddHook(failGroupInitHook{})
	// Both the XINFO GROUPS probe and the XGROUP CREATE fail: the group is
	// not created, the failure is logged (warn), and startup continues
	// without a panic.
	w.ensureGroups(context.Background())
	if w.groupExists(context.Background(), w.streamEvents) {
		t.Fatal("group should not exist after create failure")
	}
	if !strings.Contains(buf.String(), "stream group create failed") {
		t.Fatalf("create failure must be recorded, got: %s", buf.String())
	}
}

func TestGroupInit_BusyGroupRaceKeepsForeignCursor(t *testing.T) {
	w, mr := testWorker(t)
	ctx := context.Background()
	id1 := seedEvent(t, w, "u1")
	seedEvent(t, w, "u2")
	// Another instance already created the group mid-backlog.
	w.rdb.XGroupCreateMkStream(ctx, w.streamEvents, w.group, id1)
	w.rdb.Set(ctx, w.checkpointPrefix+":"+w.streamEvents, id1, 0)

	w2 := testWorkerOn(t, mr)
	w2.ensureGroups(ctx)
	// The BUSYGROUP race keeps the existing cursor: no checkpoint adoption.
	if cur := groupCursor(t, w2, w.streamEvents); cur != id1 {
		t.Fatalf("raced group cursor moved to %q, want %q", cur, id1)
	}
}

// Restart with an existing group must not regress the persisted checkpoint
// when a reclaim ACKs an older pending id (adoptCheckpoint floor).
func TestGroupInit_AdoptedCheckpointFloorsAdvance(t *testing.T) {
	w, mr := testWorker(t)
	ctx := context.Background()
	id1 := seedEvent(t, w, "u1")
	id2 := seedEvent(t, w, "u2")
	w.ensureGroups(ctx)
	readNew(t, w, w.streamEvents)
	w.advanceCheckpoint(ctx, w.streamEvents, id2)

	// Restart: the checkpoint (id2) is adopted as the floor, so advancing to
	// the older id1 cannot persist it.
	w2 := testWorkerOn(t, mr)
	w2.ensureGroups(ctx)
	w2.advanceCheckpoint(ctx, w2.streamEvents, id1)
	if got := w2.rdb.Get(ctx, w2.checkpointPrefix+":"+w2.streamEvents).Val(); got != id2 {
		t.Fatalf("checkpoint regressed to %q, want %q", got, id2)
	}
}
