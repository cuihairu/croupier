package worker

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	redis "github.com/redis/go-redis/v9"
)

// DATA-052: requeue and dead-letter moves are idempotent state changes.
// Each move is ONE atomic Lua script — XADD the target, SET a done-marker,
// XACK the source message — so there is no window between "target written"
// and "source ACKed": a script runtime error aborts before the XACK (the
// message stays pending and reclaim retries cleanly), a re-run after an
// unknown client timeout or a concurrent worker sees the marker and reduces
// to the ACK alone, and the single-threaded script registry makes concurrent
// workers serialize. The marker TTL only bounds key growth: by the time the
// marker exists the source message is already ACKed by the same script, so a
// later expiry cannot resurrect the transfer.
//
// ARGV layout for both scripts: [1]=group [2]=msgID [3]=marker TTL ms, then
// the target entry fields. miniredis's Lua bridge accepts only the canonical
// `MAXLEN ~ <n>` spelling for approximate XADD caps.
const (
	transferMarkerTTL = 24 * time.Hour

	// KEYS: [1]=marker [2]=source [3]=dead stream. Fields: the fixed
	// dead-entry shape (original_stream, original_id, reason, details,
	// failed_at, retry_count, original_data) → ARGV[4]..ARGV[17].
	// The final XACK runs under pcall: a missing group has no PEL, so there
	// is nothing left to lose (and miniredis NOGROUPs where real Redis
	// returns 0) — the transfer itself has already landed.
	deadTransferScript = `
if redis.call('EXISTS', KEYS[1]) == 1 then
  redis.pcall('XACK', KEYS[2], ARGV[1], ARGV[2])
  return 0
end
redis.call('XADD', KEYS[3], 'MAXLEN', '~', '10000', '*', ARGV[4], ARGV[5], ARGV[6], ARGV[7], ARGV[8], ARGV[9], ARGV[10], ARGV[11], ARGV[12], ARGV[13], ARGV[14], ARGV[15], ARGV[16], ARGV[17])
redis.call('SET', KEYS[1], '1', 'PX', ARGV[3])
redis.pcall('XACK', KEYS[2], ARGV[1], ARGV[2])
return 1
`

	// KEYS: [1]=marker [2]=source [3]=source. Fields: the single re-marshaled
	// retry payload → ARGV[4]=field [5]=value. No cap (DATA-019): the source
	// stream is the publisher's retention domain.
	retryTransferScript = `
if redis.call('EXISTS', KEYS[1]) == 1 then
  redis.pcall('XACK', KEYS[2], ARGV[1], ARGV[2])
  return 0
end
redis.call('XADD', KEYS[3], '*', ARGV[4], ARGV[5])
redis.call('SET', KEYS[1], '1', 'PX', ARGV[3])
redis.pcall('XACK', KEYS[2], ARGV[1], ARGV[2])
return 1
`
)

// sendToDeadLetter moves the message into the dead stream and ACKs the
// source in one atomic, idempotent script (DATA-052): a failed XAdd aborts
// the script before the ACK (message stays pending for reclaim), and a
// repeated attempt for an already-transferred message only re-ACKs.
func (w *Worker) sendToDeadLetter(ctx context.Context, stream string, msg redis.XMessage, reason, details string) error {
	deadStream := w.deadEventsStream
	if stream == w.streamPayments {
		deadStream = w.deadPaymentsStream
	}

	marker := fmt.Sprintf("analytics:transfer:dead:%s:%s", stream, msg.ID)
	args := []any{
		w.group, msg.ID, transferMarkerTTL.Milliseconds(),
		"original_stream", stream,
		"original_id", msg.ID,
		"reason", reason,
		"details", details,
		"failed_at", time.Now().Unix(),
		"retry_count", fmtAny(msg.Values["retry_count"]),
		"original_data", fmtAny(msg.Values["data"]),
	}
	if _, err := w.rdb.Eval(ctx, deadTransferScript, []string{marker, stream, deadStream}, args...).Result(); err != nil {
		slog.Error("failed to send to dead letter", "stream", deadStream, "original_id", msg.ID, "err", err)
		return err
	}
	w.advanceCheckpoint(ctx, stream, msg.ID)
	slog.Warn("message sent to dead letter", "stream", deadStream, "original_stream", stream, "original_id", msg.ID, "reason", reason)
	return nil
}

// requeueMessage re-enqueues the retry payload on the source stream and ACKs
// the original message in one atomic, idempotent script (DATA-052): a failed
// XAdd aborts before the ACK (message stays pending for reclaim), and a
// repeated attempt never writes a second retry copy.
func (w *Worker) requeueMessage(ctx context.Context, stream string, msg redis.XMessage, retryData []byte, retries int) error {
	marker := fmt.Sprintf("analytics:transfer:retry:%s:%s", stream, msg.ID)
	args := []any{w.group, msg.ID, transferMarkerTTL.Milliseconds(), "data", string(retryData)}
	if _, err := w.rdb.Eval(ctx, retryTransferScript, []string{marker, stream, stream}, args...).Result(); err != nil {
		return err
	}
	w.advanceCheckpoint(ctx, stream, msg.ID)
	return nil
}
