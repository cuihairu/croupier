package cmd

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/cuihairu/croupier/internal/analytics/mq"
	redis "github.com/redis/go-redis/v9"
	"github.com/spf13/cobra"
)

// server implements the ingestion HTTP handlers.
type server struct {
	q         mq.Queue
	secret    string
	allowSkew time.Duration
	perSecret map[string]string
	dedupe    redis.Cmdable
	dedupTTL  time.Duration
}

var (
	Version   = "dev"
	GitCommit = "unknown"
	BuildTime = ""

	listenAddr   string
	sharedSecret string
	skewSeconds  int
)

var rootCmd = &cobra.Command{
	Use:   "croupier-ingest",
	Short: "Croupier Analytics Ingest 服务",
	Long: `极简 Analytics Intake 服务，负责校验签名并将事件写入 MQ。
支持通过命令行参数覆盖监听地址、签名密钥和时间偏移。`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runIngest()
	},
}

// Execute runs the CLI entrypoint.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.Version = shortVersion()
	rootCmd.SetVersionTemplate("{{printf \"%s\\n\" .Version}}")

	rootCmd.PersistentFlags().StringVar(&listenAddr, "addr", envOrDefault("INGEST_ADDR", ":8088"), "HTTP 监听地址 (默认读取 INGEST_ADDR)")
	rootCmd.PersistentFlags().StringVar(&sharedSecret, "secret", envOrDefault("ANALYTICS_INGEST_SECRET", ""), "签名校验密钥 (默认读取 ANALYTICS_INGEST_SECRET)")
	rootCmd.PersistentFlags().IntVar(&skewSeconds, "skew", envOrDefaultInt("ANALYTICS_INGEST_SKEW", 300), "允许的时间戳偏差 (秒)")

	rootCmd.AddCommand(versionCmd)
}

func runIngest() error {
	q := mq.NewFromEnv()
	defer func() { _ = q.Close() }()

	secret := strings.TrimSpace(sharedSecret)
	if secret == "" {
		log.Println("[ingest] WARN: ANALYTICS_INGEST_SECRET/--secret 未设置，请求将被拒绝")
	}

	perGameSecrets := loadPerGameSecrets()
	dedupeClient := newDedupeClient()
	dedupeTTL := envDuration("ANALYTICS_INGEST_DEDUPE_TTL", 15*time.Minute)

	s := &server{
		q:         q,
		secret:    secret,
		allowSkew: time.Duration(skewSeconds) * time.Second,
		perSecret: perGameSecrets,
		dedupe:    dedupeClient,
		dedupTTL:  dedupeTTL,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.Handle("/api/ingest/events", s.authMiddleware(http.HandlerFunc(s.ingestEvents)))
	mux.Handle("/api/ingest/payments", s.authMiddleware(http.HandlerFunc(s.ingestPayments)))

	addr := strings.TrimSpace(listenAddr)
	if addr == "" {
		addr = ":8088"
	}

	log.Printf("[ingest] listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		return fmt.Errorf("run: %w", err)
	}
	return nil
}

// authMiddleware 校验时间戳/nonce/签名，防止重放。签名: base64(HMAC_SHA256(secret, ts + "\n" + nonce + "\n" + sha256(body))).
func (s *server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.secret == "" && len(s.perSecret) == 0 {
			respondJSON(w, http.StatusForbidden, map[string]string{"error": "ingest_disabled"})
			return
		}
		gameID := strings.TrimSpace(r.Header.Get("X-Game-Id"))
		secret := s.secretForGame(gameID)
		if secret == "" {
			respondJSON(w, http.StatusForbidden, map[string]string{"error": "unknown_game"})
			return
		}
		tsStr := strings.TrimSpace(r.Header.Get("X-Timestamp"))
		nonce := strings.TrimSpace(r.Header.Get("X-Nonce"))
		sig := strings.TrimSpace(r.Header.Get("X-Signature"))
		if tsStr == "" || nonce == "" || sig == "" {
			respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "missing_auth_headers"})
			return
		}
		ts, err := strconv.ParseInt(tsStr, 10, 64)
		if err != nil {
			respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "bad_timestamp"})
			return
		}
		now := time.Now().Unix()
		if delta := time.Duration(abs64(now-ts)) * time.Second; delta > s.allowSkew {
			respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "timestamp_skew"})
			return
		}
		// hash body
		body, err := io.ReadAll(r.Body)
		if err != nil {
			respondJSON(w, http.StatusBadRequest, map[string]string{"error": "read_body_failed"})
			return
		}
		sum := sha256.Sum256(body)
		sumHex := hex.EncodeToString(sum[:])
		msg := tsStr + "\n" + nonce + "\n" + sumHex
		mac := hmac.New(sha256.New, []byte(secret))
		_, _ = mac.Write([]byte(msg))
		expected := base64.StdEncoding.EncodeToString(mac.Sum(nil))
		if !hmac.Equal([]byte(expected), []byte(sig)) {
			respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "bad_signature"})
			return
		}
		if s.dedupe != nil {
			if exists, err := s.dedupe.Exists(r.Context(), s.nonceKey(gameID, nonce)).Result(); err == nil && exists > 0 {
				respondJSON(w, http.StatusTooManyRequests, map[string]string{"error": "duplicate_nonce"})
				return
			}
			if err := s.dedupe.SetEx(r.Context(), s.nonceKey(gameID, nonce), 1, s.dedupTTL).Err(); err != nil {
				log.Printf("[ingest] warn: dedupe set failed: %v", err)
			}
		}
		// Put back body for next handler
		r.Body = io.NopCloser(bytes.NewReader(body))
		r.Header.Set("X-Game-Id", gameID)
		next.ServeHTTP(w, r)
	})
}

// ingestEvents 接收通用事件数组，写入 MQ: analytics:events
func (s *server) ingestEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var arr []map[string]any
	if err := json.NewDecoder(r.Body).Decode(&arr); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_payload"})
		return
	}
	gameID := strings.TrimSpace(r.Header.Get("X-Game-Id"))
	for _, e := range arr {
		if err := validateEventPayload(e, gameID); err != nil {
			respondJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		if err := s.q.PublishEvent(e); err != nil {
			respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "queue_write_failed"})
			return
		}
	}
	w.WriteHeader(http.StatusAccepted)
}

// ingestPayments 接收支付事件数组，写入 MQ: analytics:payments
func (s *server) ingestPayments(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var arr []map[string]any
	if err := json.NewDecoder(r.Body).Decode(&arr); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_payload"})
		return
	}
	gameID := strings.TrimSpace(r.Header.Get("X-Game-Id"))
	for _, e := range arr {
		if err := validatePaymentPayload(e, gameID); err != nil {
			respondJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		if err := s.q.PublishPayment(e); err != nil {
			respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "queue_write_failed"})
			return
		}
	}
	w.WriteHeader(http.StatusAccepted)
}

func abs64(x int64) int64 {
	if x < 0 {
		return -x
	}
	return x
}

func respondJSON(w http.ResponseWriter, code int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(data)
}

func (s *server) secretForGame(gameID string) string {
	if gameID != "" {
		if sec := s.perSecret[gameID]; sec != "" {
			return sec
		}
	}
	return s.secret
}

func (s *server) nonceKey(gameID, nonce string) string {
	if gameID == "" {
		gameID = "default"
	}
	return fmt.Sprintf("ingest:nonce:%s:%s", gameID, nonce)
}

func validateEventPayload(evt map[string]any, headerGame string) error {
	required := []string{"game_id", "env", "ts", "event"}
	for _, k := range required {
		if mapString(evt, k) == "" {
			return fmt.Errorf("missing %s", k)
		}
	}
	if headerGame != "" && mapString(evt, "game_id") != headerGame {
		return fmt.Errorf("game_id mismatch")
	}
	return nil
}

func validatePaymentPayload(pay map[string]any, headerGame string) error {
	required := []string{"game_id", "env", "ts", "order_id", "status"}
	for _, k := range required {
		if mapString(pay, k) == "" {
			return fmt.Errorf("missing %s", k)
		}
	}
	if headerGame != "" && mapString(pay, "game_id") != headerGame {
		return fmt.Errorf("game_id mismatch")
	}
	if mapFloat(pay, "amount_cents") <= 0 {
		return fmt.Errorf("amount_cents required")
	}
	return nil
}

func loadPerGameSecrets() map[string]string {
	out := map[string]string{}
	raw := strings.TrimSpace(os.Getenv("ANALYTICS_INGEST_SECRETS"))
	if raw == "" {
		return out
	}
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		log.Printf("[ingest] WARN: failed to parse ANALYTICS_INGEST_SECRETS: %v", err)
		return map[string]string{}
	}
	return out
}

func newDedupeClient() redis.Cmdable {
	url := strings.TrimSpace(os.Getenv("INGEST_REDIS_URL"))
	if url == "" {
		url = strings.TrimSpace(os.Getenv("REDIS_URL"))
	}
	if url == "" {
		return nil
	}
	opt, err := redis.ParseURL(url)
	if err != nil {
		log.Printf("[ingest] WARN: failed to parse INGEST_REDIS_URL: %v", err)
		return nil
	}
	return redis.NewClient(opt)
}

func envDuration(key string, def time.Duration) time.Duration {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return def
}

func mapString(m map[string]any, key string) string {
	if v, ok := m[key]; ok {
		switch t := v.(type) {
		case string:
			return t
		case []byte:
			return string(t)
		}
	}
	return ""
}

func mapFloat(m map[string]any, key string) float64 {
	if v, ok := m[key]; ok {
		switch t := v.(type) {
		case float64:
			return t
		case int:
			return float64(t)
		case int64:
			return float64(t)
		}
	}
	return 0
}

func shortVersion() string {
	switch {
	case GitCommit == "unknown" && BuildTime == "":
		return Version
	case BuildTime == "":
		return fmt.Sprintf("%s (%s)", Version, GitCommit)
	default:
		return fmt.Sprintf("%s (%s, built %s)", Version, GitCommit, BuildTime)
	}
}

func envOrDefault(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

func envOrDefaultInt(key string, def int) int {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return def
}
