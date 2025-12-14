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
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cuihairu/croupier/internal/analytics/mq"
	redis "github.com/redis/go-redis/v9"
	"github.com/spf13/cobra"
	"golang.org/x/time/rate"
)

// server implements the ingestion HTTP handlers.
type server struct {
	q         mq.Queue
	secret    string
	allowSkew time.Duration
	perSecret map[string]string
	dedupe    redis.Cmdable
	dedupTTL  time.Duration

	// Rate limiting
	limiter   *rate.Limiter
	rateLimit int // requests per second
	rateBurst int // burst size

	// Metrics
	requestsTotal   int64
	requestsSuccess int64
	requestsError   int64
	requestsDropped int64

	// Detailed metrics
	eventsProcessed   int64
	paymentsProcessed int64
	validationErrors  int64
	queueErrors       int64
	authErrors        int64

	// Latency metrics (microseconds)
	latencySum   int64
	latencyCount int64

	// Queue metrics
	queueLastCheck  time.Time
	eventsPending   int64
	paymentsPending int64

	// Configuration
	maxBodySize  int64 // maximum body size in bytes
	readTimeout  time.Duration
	writeTimeout time.Duration
	idleTimeout  time.Duration

	// Metrics collection
	mu sync.RWMutex
}

var (
	Version   = "dev"
	GitCommit = "unknown"
	BuildTime = ""

	listenAddr   string
	sharedSecret string
	skewSeconds  int

	// Rate limiting
	rateLimitRPS int
	rateBurst    int

	// HTTP timeouts
	readTimeoutSec  int
	writeTimeoutSec int
	idleTimeoutSec  int

	// Body size limit
	maxBodySizeMB int
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

	// Rate limiting flags
	rootCmd.PersistentFlags().IntVar(&rateLimitRPS, "rate-limit", envOrDefaultInt("ANALYTICS_INGEST_RATE_LIMIT", 1000), "每秒请求限制 (RPS)")
	rootCmd.PersistentFlags().IntVar(&rateBurst, "rate-burst", envOrDefaultInt("ANALYTICS_INGEST_RATE_BURST", 100), "突发请求限制")

	// HTTP timeout flags
	rootCmd.PersistentFlags().IntVar(&readTimeoutSec, "read-timeout", envOrDefaultInt("ANALYTICS_INGEST_READ_TIMEOUT", 10), "读取超时 (秒)")
	rootCmd.PersistentFlags().IntVar(&writeTimeoutSec, "write-timeout", envOrDefaultInt("ANALYTICS_INGEST_WRITE_TIMEOUT", 10), "写入超时 (秒)")
	rootCmd.PersistentFlags().IntVar(&idleTimeoutSec, "idle-timeout", envOrDefaultInt("ANALYTICS_INGEST_IDLE_TIMEOUT", 60), "空闲超时 (秒)")

	// Body size limit
	rootCmd.PersistentFlags().IntVar(&maxBodySizeMB, "max-body-size", envOrDefaultInt("ANALYTICS_INGEST_MAX_BODY_MB", 10), "最大请求体大小 (MB)")

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

	// Initialize rate limiter
	rateLimiter := rate.NewLimiter(rate.Limit(rateLimitRPS), rateBurst)

	// Convert MB to bytes
	maxBodySize := int64(maxBodySizeMB) * 1024 * 1024

	s := &server{
		q:         q,
		secret:    secret,
		allowSkew: time.Duration(skewSeconds) * time.Second,
		perSecret: perGameSecrets,
		dedupe:    dedupeClient,
		dedupTTL:  dedupeTTL,

		// Rate limiting
		limiter:   rateLimiter,
		rateLimit: rateLimitRPS,
		rateBurst: rateBurst,

		// Configuration
		maxBodySize:  maxBodySize,
		readTimeout:  time.Duration(readTimeoutSec) * time.Second,
		writeTimeout: time.Duration(writeTimeoutSec) * time.Second,
		idleTimeout:  time.Duration(idleTimeoutSec) * time.Second,
	}

	mux := http.NewServeMux()

	// Add metrics endpoint
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		s.mu.RLock()
		eventsPending := atomic.LoadInt64(&s.eventsPending)
		paymentsPending := atomic.LoadInt64(&s.paymentsPending)
		s.mu.RUnlock()

		// Calculate average latency
		latencySum := atomic.LoadInt64(&s.latencySum)
		latencyCount := atomic.LoadInt64(&s.latencyCount)
		avgLatencyMs := float64(0)
		if latencyCount > 0 {
			avgLatencyMs = float64(latencySum) / float64(latencyCount) / 1000 // Convert to milliseconds
		}

		// Get memory stats
		var m runtime.MemStats
		runtime.ReadMemStats(&m)

		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":    "ok",
			"version":   Version,
			"timestamp": time.Now().Unix(),
			"metrics": map[string]interface{}{
				"requests_total":     atomic.LoadInt64(&s.requestsTotal),
				"requests_success":   atomic.LoadInt64(&s.requestsSuccess),
				"requests_error":     atomic.LoadInt64(&s.requestsError),
				"requests_dropped":   atomic.LoadInt64(&s.requestsDropped),
				"events_processed":   atomic.LoadInt64(&s.eventsProcessed),
				"payments_processed": atomic.LoadInt64(&s.paymentsProcessed),
				"validation_errors":  atomic.LoadInt64(&s.validationErrors),
				"queue_errors":       atomic.LoadInt64(&s.queueErrors),
				"auth_errors":        atomic.LoadInt64(&s.authErrors),
				"avg_latency_ms":     avgLatencyMs,
				"events_pending":     eventsPending,
				"payments_pending":   paymentsPending,
			},
			"system": map[string]interface{}{
				"goroutines":   runtime.NumGoroutine(),
				"mem_alloc_mb": m.Alloc / 1024 / 1024,
				"mem_total_mb": m.TotalAlloc / 1024 / 1024,
				"mem_sys_mb":   m.Sys / 1024 / 1024,
				"gc_cycles":    m.NumGC,
			},
			"config": map[string]interface{}{
				"rate_limit_rps":   rateLimitRPS,
				"rate_burst":       rateBurst,
				"max_body_size_mb": maxBodySizeMB,
				"read_timeout_s":   readTimeoutSec,
				"write_timeout_s":  writeTimeoutSec,
				"idle_timeout_s":   idleTimeoutSec,
			},
		})
	})

	// Apply middleware chain
	eventsHandler := s.metricsMiddleware(s.rateLimitMiddleware(http.HandlerFunc(s.ingestEvents)))
	paymentsHandler := s.metricsMiddleware(s.rateLimitMiddleware(http.HandlerFunc(s.ingestPayments)))

	mux.Handle("/api/ingest/events", s.authMiddleware(eventsHandler))
	mux.Handle("/api/ingest/payments", s.authMiddleware(paymentsHandler))

	// Configure HTTP server with timeouts
	server := &http.Server{
		Addr:         strings.TrimSpace(listenAddr),
		Handler:      mux,
		ReadTimeout:  s.readTimeout,
		WriteTimeout: s.writeTimeout,
		IdleTimeout:  s.idleTimeout,
	}

	addr := server.Addr
	if addr == "" {
		addr = ":8088"
	}

	log.Printf("[ingest] listening on %s (rate: %d rps, burst: %d, max body: %d MB)",
		addr, rateLimitRPS, rateBurst, maxBodySizeMB)

	// Start metrics reporter in background
	go s.reportMetrics()

	// Start queue monitoring
	go s.monitorQueueBacklog()

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
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

// rateLimitMiddleware implements rate limiting
func (s *server) rateLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.limiter.Allow() {
			atomic.AddInt64(&s.requestsDropped, 1)
			respondJSON(w, http.StatusTooManyRequests, map[string]string{
				"error":       "rate_limit_exceeded",
				"limit":       fmt.Sprintf("%d_rps", s.rateLimit),
				"retry_after": "1s",
			})
			return
		}
		next.ServeHTTP(w, r)
	})
}

// metricsMiddleware tracks request metrics
func (s *server) metricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		atomic.AddInt64(&s.requestsTotal, 1)

		// Create a response writer to capture status code
		rw := &responseWriter{ResponseWriter: w}

		// Limit body size before reading
		// Note: We limit after auth to ensure we can track metrics
		// In production, you might want to use http.MaxBytesReader
		if r.ContentLength > 0 && r.ContentLength > s.maxBodySize {
			atomic.AddInt64(&s.requestsError, 1)
			respondJSON(rw, http.StatusRequestEntityTooLarge, map[string]string{
				"error":       "body_too_large",
				"max_size_mb": fmt.Sprintf("%d", maxBodySizeMB),
				"actual_size": fmt.Sprintf("%d", r.ContentLength/1024/1024),
			})
			return
		}

		// Serve the request
		// Note: In a real implementation, you might want to replace r.Body with a limited reader
		// For now, we'll read the body and check size in the handlers
		next.ServeHTTP(rw, r)

		// Record latency in microseconds
		duration := time.Since(start)
		durationMicros := duration.Microseconds()
		atomic.AddInt64(&s.latencySum, durationMicros)
		atomic.AddInt64(&s.latencyCount, 1)

		// Record metrics based on status code
		if rw.status >= 200 && rw.status < 400 {
			atomic.AddInt64(&s.requestsSuccess, 1)
		} else {
			atomic.AddInt64(&s.requestsError, 1)
			// Track specific error types
			if rw.status == 400 {
				atomic.AddInt64(&s.validationErrors, 1)
			} else if rw.status == 401 || rw.status == 403 {
				atomic.AddInt64(&s.authErrors, 1)
			} else if rw.status >= 500 {
				atomic.AddInt64(&s.queueErrors, 1)
			}
		}

		// Log slow requests
		if duration > time.Second {
			log.Printf("[ingest] slow request: %s %s took %v (status: %d)", r.Method, r.URL.Path, duration, rw.status)
		}
	})
}

// responseWriter wraps http.ResponseWriter to capture status code
type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

// ingestEvents 接收通用事件数组，写入 MQ: analytics:events
func (s *server) ingestEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Read body with size limit
	limitedReader := http.MaxBytesReader(w, r.Body, int64(s.maxBodySize))
	var arr []map[string]any
	if err := json.NewDecoder(limitedReader).Decode(&arr); err != nil {
		// Check if it's a size error
		if err.Error() == "http: request body too large" {
			respondJSON(w, http.StatusRequestEntityTooLarge, map[string]string{
				"error":       "body_too_large",
				"max_size_mb": fmt.Sprintf("%d", maxBodySizeMB),
			})
			return
		}
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
		atomic.AddInt64(&s.eventsProcessed, 1)
	}
	w.WriteHeader(http.StatusAccepted)
}

// ingestPayments 接收支付事件数组，写入 MQ: analytics:payments
func (s *server) ingestPayments(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Read body with size limit
	limitedReader := http.MaxBytesReader(w, r.Body, int64(s.maxBodySize))
	var arr []map[string]any
	if err := json.NewDecoder(limitedReader).Decode(&arr); err != nil {
		// Check if it's a size error
		if err.Error() == "http: request body too large" {
			respondJSON(w, http.StatusRequestEntityTooLarge, map[string]string{
				"error":       "body_too_large",
				"max_size_mb": fmt.Sprintf("%d", maxBodySizeMB),
			})
			return
		}
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
		atomic.AddInt64(&s.paymentsProcessed, 1)
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

// monitorQueueBacklog periodically checks queue backlog
func (s *server) monitorQueueBacklog() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		if mq, ok := s.q.(interface {
			PendingEvents() (int64, error)
			PendingPayments() (int64, error)
		}); ok {
			if events, err := mq.PendingEvents(); err == nil {
				atomic.StoreInt64(&s.eventsPending, events)
			}
			if payments, err := mq.PendingPayments(); err == nil {
				atomic.StoreInt64(&s.paymentsPending, payments)
			}
		}
		s.queueLastCheck = time.Now()
	}
}

// reportMetrics periodically reports detailed metrics
func (s *server) reportMetrics() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		total := atomic.LoadInt64(&s.requestsTotal)
		success := atomic.LoadInt64(&s.requestsSuccess)
		error := atomic.LoadInt64(&s.requestsError)
		dropped := atomic.LoadInt64(&s.requestsDropped)

		events := atomic.LoadInt64(&s.eventsProcessed)
		payments := atomic.LoadInt64(&s.paymentsProcessed)
		validationErrs := atomic.LoadInt64(&s.validationErrors)
		queueErrs := atomic.LoadInt64(&s.queueErrors)
		authErrs := atomic.LoadInt64(&s.authErrors)

		// Calculate QPS
		qps := float64(total) / 30.0

		// Calculate average latency
		latencySum := atomic.LoadInt64(&s.latencySum)
		latencyCount := atomic.LoadInt64(&s.latencyCount)
		avgLatencyMs := float64(0)
		if latencyCount > 0 {
			avgLatencyMs = float64(latencySum) / float64(latencyCount) / 1000
		}

		// Get queue backlog
		s.mu.RLock()
		eventsPending := atomic.LoadInt64(&s.eventsPending)
		paymentsPending := atomic.LoadInt64(&s.paymentsPending)
		s.mu.RUnlock()

		// Log detailed metrics
		log.Printf("[ingest] metrics - qps: %.2f, total: %d, success: %d, error: %d, dropped: %d",
			qps, total, success, error, dropped)
		log.Printf("[ingest] events - processed: %d, pending: %d", events, eventsPending)
		log.Printf("[ingest] payments - processed: %d, pending: %d", payments, paymentsPending)
		log.Printf("[ingest] errors - validation: %d, queue: %d, auth: %d", validationErrs, queueErrs, authErrs)
		log.Printf("[ingest] latency - avg: %.2fms, samples: %d", avgLatencyMs, latencyCount)

		// Alert on high error rate
		if total > 0 {
			errorRate := float64(error) / float64(total) * 100
			if errorRate > 10 { // Alert if error rate > 10%
				log.Printf("[ingest] ALERT: High error rate: %.2f%%", errorRate)
			}
		}

		// Alert on queue backlog
		if eventsPending > 10000 {
			log.Printf("[ingest] ALERT: High events backlog: %d", eventsPending)
		}
		if paymentsPending > 10000 {
			log.Printf("[ingest] ALERT: High payments backlog: %d", paymentsPending)
		}

		// Alert on high latency
		if avgLatencyMs > 1000 { // Alert if avg latency > 1 second
			log.Printf("[ingest] ALERT: High latency: %.2fms", avgLatencyMs)
		}

		// Reset counters periodically to avoid overflow
		if latencyCount > 1000000 {
			atomic.StoreInt64(&s.latencySum, 0)
			atomic.StoreInt64(&s.latencyCount, 0)
		}
	}
}
