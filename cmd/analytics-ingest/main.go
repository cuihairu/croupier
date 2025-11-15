package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
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

	r := gin.New()
	r.Use(gin.Recovery())
	r.GET("/healthz", func(c *gin.Context) { c.String(200, "ok") })

	api := r.Group("/api/ingest", s.authMiddleware)
	api.POST("/events", s.ingestEvents)
	api.POST("/payments", s.ingestPayments)

	addr := strings.TrimSpace(os.Getenv("INGEST_ADDR"))
	if addr == "" {
		addr = ":8088"
	}
	log.Printf("[ingest] listening on %s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("run: %v", err)
	}
}

// authMiddleware 校验时间戳/nonce/签名，防止重放。签名: base64(HMAC_SHA256(secret, ts + "\n" + nonce + "\n" + sha256(body))).
func (s *server) authMiddleware(c *gin.Context) {
	if s.secret == "" {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "ingest_disabled"})
		return
	}
	tsStr := strings.TrimSpace(c.GetHeader("X-Timestamp"))
	nonce := strings.TrimSpace(c.GetHeader("X-Nonce"))
	sig := strings.TrimSpace(c.GetHeader("X-Signature"))
	if tsStr == "" || nonce == "" || sig == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing_auth_headers"})
		return
	}
	ts, err := strconv.ParseInt(tsStr, 10, 64)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "bad_timestamp"})
		return
	}
	now := time.Now().Unix()
	if delta := time.Duration(abs64(now-ts)) * time.Second; delta > s.allowSkew {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "timestamp_skew"})
		return
	}
	// hash body
	body, err := c.GetRawData()
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "read_body_failed"})
		return
	}
	sum := sha256.Sum256(body)
	sumHex := hex.EncodeToString(sum[:])
	msg := tsStr + "\n" + nonce + "\n" + sumHex
	mac := hmac.New(sha256.New, []byte(s.secret))
	_, _ = mac.Write([]byte(msg))
	expected := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(expected), []byte(sig)) {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "bad_signature"})
		return
	}
	// Put back body for next handler
	c.Request.Body = newReadCloser(body)
	c.Next()
}

// ingestEvents 接收通用事件数组，写入 MQ: analytics:events
func (s *server) ingestEvents(c *gin.Context) {
	var arr []map[string]any
	if err := c.BindJSON(&arr); err != nil {
		c.JSON(400, gin.H{"error": "invalid_payload"})
		return
	}
	for _, e := range arr {
		if err := s.q.PublishEvent(e); err != nil {
			c.JSON(500, gin.H{"error": "queue_write_failed"})
			return
		}
	}
	c.Status(202)
}

// ingestPayments 接收支付事件数组，写入 MQ: analytics:payments
func (s *server) ingestPayments(c *gin.Context) {
	var arr []map[string]any
	if err := c.BindJSON(&arr); err != nil {
		c.JSON(400, gin.H{"error": "invalid_payload"})
		return
	}
	for _, e := range arr {
		if err := s.q.PublishPayment(e); err != nil {
			c.JSON(500, gin.H{"error": "queue_write_failed"})
			return
		}
	}
	c.Status(202)
}

// Helpers

func abs64(x int64) int64 {
	if x < 0 {
		return -x
	}
	return x
}

type rc struct{ b []byte }

func newReadCloser(b []byte) *rc { return &rc{b: b} }
func (r *rc) Read(p []byte) (int, error) {
	n := copy(p, r.b)
	r.b = r.b[n:]
	if n == 0 {
		return 0, io.EOF
	}
	return n, nil
}
func (r *rc) Close() error { return nil }
