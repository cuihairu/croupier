// Crash aggregation (bug-tracking P2; see docs/research/bug-tracking-design.md
// §4 P2: same-stack crash reports aggregate into one bug with a counter).
package bug

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/model"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// crashFingerprintKey is the Extra field carrying the aggregation key.
const crashFingerprintKey = "crashFingerprint"
const crashCountKey = "crashCount"
const crashLastSeenKey = "crashLastSeenAt"
const crashLastPlayerKey = "crashLastPlayerId"

// ReportCrashRequest files one raw crash report (game client SDK or agent
// reporter). The stack is fingerprinted; the first report for a fingerprint
// creates a triage bug, subsequent reports only bump the counter.
type ReportCrashRequest struct {
	GameID     string `json:"gameId"`
	Env        string `json:"env,optional"`
	Platform   string `json:"platform,optional"`
	PlayerID   string `json:"playerId,optional"`
	ServerID   string `json:"serverId,optional"`
	Device     string `json:"device,optional"`
	OS         string `json:"deviceOs,optional"`
	AppVersion string `json:"appVersion,optional"`
	// Stack is the raw stack trace text (required).
	Stack string `json:"stack"`
	// Message is the one-line error summary (optional; derived from stack).
	Message string `json:"message,optional"`
}

// ReportCrashResponse returns the aggregated bug and its total count.
type ReportCrashResponse struct {
	BugID       int64  `json:"bugId"`
	Count       int64  `json:"count"`
	Created     bool   `json:"created"` // true when this report opened a new bug
	Fingerprint string `json:"fingerprint"`
}

// Fingerprint noise patterns: hex addresses (0x7ffd…), +offset suffixes and
// :line numbers anywhere in a frame. Applied globally so "file.lua:123:" and
// "file.lua:321:" normalize identically.
var (
	reHexAddr = regexp.MustCompile(`0x[0-9a-fA-F]+`)
	reOffset  = regexp.MustCompile(`\+\s*0x[0-9a-fA-F]+`)
	reLineNum = regexp.MustCompile(`:\d+`)
	reSpaces  = regexp.MustCompile(`\s+`)
)

// fingerprintStack normalizes a stack trace into a stable aggregation key:
// strip addresses/offsets/line numbers so the same crash site maps to the
// same fingerprint regardless of build noise.
func fingerprintStack(stack string) string {
	lines := strings.Split(strings.TrimSpace(stack), "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		line = reOffset.ReplaceAllString(line, "")
		line = reHexAddr.ReplaceAllString(line, "")
		line = reLineNum.ReplaceAllString(line, "")
		line = reSpaces.ReplaceAllString(line, " ")
		out = append(out, line)
	}
	normalized := strings.Join(out, "\n")
	sum := sha256.Sum256([]byte(strings.ToLower(normalized)))
	return hex.EncodeToString(sum[:])[:16]
}

// ReportCrash ingests one crash report with aggregation.
func (s *Service) ReportCrash(ctx context.Context, req *ReportCrashRequest) (*ReportCrashResponse, error) {
	stack := strings.TrimSpace(req.Stack)
	if stack == "" {
		return nil, errorx.NewBadRequest("stack 不能为空")
	}
	if strings.TrimSpace(req.GameID) == "" {
		return nil, errorx.NewBadRequest("gameId 不能为空")
	}
	fingerprint := fingerprintStack(stack)

	// Look up an existing open bug with the same fingerprint (indexed
	// column; terminal bugs no longer aggregate and a fresh one is opened).
	if existing, err := s.svcCtx.BugModel.FindOpenByCrashFingerprint(ctx,
		strings.TrimSpace(req.GameID), strings.TrimSpace(req.Env), fingerprint); err == nil {
		return s.bumpCrashBug(ctx, existing, req, fingerprint, false)
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	// First report for this fingerprint → open a new triage bug.
	title := strings.TrimSpace(req.Message)
	if title == "" {
		title = firstStackLine(stack)
		if len([]rune(title)) > 60 {
			title = string([]rune(title)[:60]) + "…"
		}
	}
	platform := strings.ToLower(strings.TrimSpace(req.Platform))
	if _, ok := model.ValidReleasePlatforms[platform]; !ok {
		platform = ""
	}
	extra := map[string]interface{}{
		crashFingerprintKey: fingerprint,
		crashCountKey:       int64(1),
		crashLastSeenKey:    time.Now().UTC().Format(time.RFC3339),
	}
	if req.PlayerID != "" {
		extra[crashLastPlayerKey] = req.PlayerID
	}
	if req.AppVersion != "" {
		extra["appVersion"] = req.AppVersion
	}
	bug := &model.Bug{
		Title:            "崩溃: " + truncateRunes(title, 200),
		Content:          "自动聚合的崩溃报告（同指纹堆栈计数见 extra.crashCount）。\n\n" + truncateRunes(stack, 8000),
		CrashFingerprint: fingerprint,
		Status:           model.BugStatusTriage,
		Severity:         model.BugSeverityCritical,
		Priority:         "high",
		GameID:           strings.TrimSpace(req.GameID),
		Env:              strings.TrimSpace(req.Env),
		ServerID:         strings.TrimSpace(req.ServerID),
		Platform:         platform,
		Device:           strings.TrimSpace(req.Device),
		OS:               strings.TrimSpace(req.OS),
		Steps:            stack,
		Source:           "player",
		PlayerID:         strings.TrimSpace(req.PlayerID),
		Extra:            extra,
	}
	if err := s.svcCtx.BugModel.Create(ctx, bug); err != nil {
		return nil, err
	}
	return &ReportCrashResponse{
		BugID: int64(bug.ID), Count: 1, Created: true, Fingerprint: fingerprint,
	}, nil
}

// bumpCrashBug increments the aggregation counter on an existing bug.
func (s *Service) bumpCrashBug(ctx context.Context, bug *model.Bug, req *ReportCrashRequest, fingerprint string, _ bool) (*ReportCrashResponse, error) {
	count, _ := toInt64(bug.Extra[crashCountKey])
	count++
	extra := bug.Extra
	if extra == nil {
		extra = map[string]interface{}{}
	}
	extra[crashCountKey] = count
	extra[crashLastSeenKey] = time.Now().UTC().Format(time.RFC3339)
	if req.PlayerID != "" {
		extra[crashLastPlayerKey] = req.PlayerID
	}
	if err := s.svcCtx.BugModel.Update(ctx, bug.ID, map[string]interface{}{
		"extra": datatypes.JSONMap(extra),
	}); err != nil {
		return nil, err
	}
	return &ReportCrashResponse{BugID: int64(bug.ID), Count: count, Fingerprint: fingerprint}, nil
}

func toInt64(v interface{}) (int64, error) {
	switch t := v.(type) {
	case int64:
		return t, nil
	case int:
		return int64(t), nil
	case float64:
		return int64(t), nil
	default:
		return 0, errors.New("not a number")
	}
}

func firstStackLine(stack string) string {
	if i := strings.IndexByte(stack, '\n'); i > 0 {
		return stack[:i]
	}
	return stack
}

func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}
