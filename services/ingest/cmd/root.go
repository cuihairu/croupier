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
	"github.com/spf13/cobra"
)

// server implements the ingestion HTTP handlers.
type server struct {
	q         mq.Queue
	secret    string
	allowSkew time.Duration
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

	s := &server{
		q:         q,
		secret:    secret,
		allowSkew: time.Duration(skewSeconds) * time.Second,
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
		if s.secret == "" {
			respondJSON(w, http.StatusForbidden, map[string]string{"error": "ingest_disabled"})
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
		mac := hmac.New(sha256.New, []byte(s.secret))
		_, _ = mac.Write([]byte(msg))
		expected := base64.StdEncoding.EncodeToString(mac.Sum(nil))
		if !hmac.Equal([]byte(expected), []byte(sig)) {
			respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "bad_signature"})
			return
		}
		// Put back body for next handler
		r.Body = io.NopCloser(bytes.NewReader(body))
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
	for _, e := range arr {
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
	for _, e := range arr {
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
