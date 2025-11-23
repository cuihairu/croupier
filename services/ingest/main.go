package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/cuihairu/croupier/internal/analytics/mq"
)

// KISS: 极简 Ingestion 服务，仅实现签名校验 + Redis Streams 写入。
// 通过环境变量配置，避免引入额外依赖或复杂性。

type server struct {
	q         mq.Queue
	secret    string
	allowSkew time.Duration
}

func main() {
	// MQ 出口（默认 noop；生产建议设置 ANALYTICS_MQ_TYPE=redis 且 REDIS_URL）
	q := mq.NewFromEnv()
	defer func() { _ = q.Close() }()

	secret := strings.TrimSpace(os.Getenv("ANALYTICS_INGEST_SECRET"))
	if secret == "" {
		log.Println("[ingest] WARN: ANALYTICS_INGEST_SECRET not set; requests will be rejected")
	}
	allowSkew := 300 * time.Second
	if v := strings.TrimSpace(os.Getenv("ANALYTICS_INGEST_SKEW")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			allowSkew = time.Duration(n) * time.Second
		}
	}

	s := &server{q: q, secret: secret, allowSkew: allowSkew}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})
	mux.Handle("/api/ingest/events", s.authMiddleware(http.HandlerFunc(s.ingestEvents)))
	mux.Handle("/api/ingest/payments", s.authMiddleware(http.HandlerFunc(s.ingestPayments)))

	addr := strings.TrimSpace(os.Getenv("INGEST_ADDR"))
	if addr == "" {
		addr = ":8088"
	}
	log.Printf("[ingest] listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("run: %v", err)
	}
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

// Helpers

func abs64(x int64) int64 {
	if x < 0 {
		return -x
	}
	return x
}

func respondJSON(w http.ResponseWriter, code int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(data)
}
