// Package quicksdk provides a client for the QuickSDK open API.
// See https://www.quicksdk.com/doc-1133.html for API documentation.
package quicksdk

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	// DefaultAPIBaseURL is the default QuickSDK API base URL.
	DefaultAPIBaseURL = "https://www.quicksdk.com"
	// DefaultTimeout is the default HTTP request timeout.
	DefaultTimeout = 30 * time.Second
	// DefaultRetryCount is the default number of retries.
	DefaultRetryCount = 3
)

// Client is a QuickSDK API client.
type Client struct {
	baseURL    string
	openID     string
	openKey    string
	httpClient *http.Client
	logger     *slog.Logger
	enabled    bool

	// Rate limiting
	rateLimiter *rateLimiter

	// Caching
	enableCache   bool
	cache         *cache
	cacheDuration time.Duration
}

// Config holds the QuickSDK client configuration.
type Config struct {
	OpenID            string        `yaml:"open_id" json:"open_id"`
	OpenKey           string        `yaml:"open_key" json:"open_key"`
	APIBaseURL        string        `yaml:"api_base_url" json:"api_base_url"`
	Timeout           time.Duration `yaml:"timeout" json:"timeout"`
	RetryCount        int           `yaml:"retry_count" json:"retry_count"`
	EnableCache       bool          `yaml:"enable_cache" json:"enable_cache"`
	CacheDuration     time.Duration `yaml:"cache_duration" json:"cache_duration"`
	RequestsPerMinute int           `yaml:"requests_per_minute" json:"requests_per_minute"`
}

// NewClient creates a new QuickSDK client.
func NewClient(config Config, logger *slog.Logger) (*Client, error) {
	if config.OpenID == "" {
		return nil, fmt.Errorf("open_id is required")
	}
	if config.OpenKey == "" {
		return nil, fmt.Errorf("open_key is required")
	}

	if config.APIBaseURL == "" {
		config.APIBaseURL = DefaultAPIBaseURL
	}
	if config.Timeout == 0 {
		config.Timeout = DefaultTimeout
	}
	if config.RetryCount == 0 {
		config.RetryCount = DefaultRetryCount
	}
	if config.CacheDuration == 0 {
		config.CacheDuration = 5 * time.Minute
	}

	var rl *rateLimiter
	if config.RequestsPerMinute > 0 {
		rl = newRateLimiter(config.RequestsPerMinute)
	}

	var c *cache
	if config.EnableCache {
		c = newCache(config.CacheDuration)
	}

	if logger == nil {
		logger = slog.Default()
	}

	return &Client{
		baseURL: config.APIBaseURL,
		openID:  config.OpenID,
		openKey: config.OpenKey,
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
		logger:        logger,
		enabled:       true,
		rateLimiter:   rl,
		enableCache:   config.EnableCache,
		cache:         c,
		cacheDuration: config.CacheDuration,
	}, nil
}

// IsEnabled returns whether the client is enabled.
func (c *Client) IsEnabled() bool {
	return c.enabled
}

// SetEnabled sets the enabled state of the client.
func (c *Client) SetEnabled(enabled bool) {
	c.enabled = enabled
}

// Close closes the client and releases resources.
func (c *Client) Close() error {
	if c.cache != nil {
		c.cache.Close()
	}
	return nil
}

// Response is the QuickSDK API response structure.
type Response struct {
	Status  bool            `json:"status"`
	Data    json.RawMessage `json:"data"`
	Message string          `json:"message"`
}

// Do performs an API call to QuickSDK.
// endpoint is the API endpoint (e.g., "open/dayReport").
// params is the request parameters (will be signed).
func (c *Client) Do(ctx context.Context, endpoint string, params map[string]interface{}) (*Response, error) {
	// Check if client is enabled
	if !c.enabled {
		return nil, fmt.Errorf("quicksdk client is disabled")
	}

	// Check cache for GET-like requests
	cacheKey := ""
	if c.enableCache && c.isCacheableRequest(endpoint, params) {
		cacheKey = c.buildCacheKey(endpoint, params)
		if cached, found := c.cache.Get(cacheKey); found {
			c.logger.Debug("cache hit", "endpoint", endpoint)
			return cached, nil
		}
	}

	// Wait for rate limiter
	if c.rateLimiter != nil {
		if err := c.rateLimiter.Wait(ctx); err != nil {
			return nil, fmt.Errorf("rate limit wait: %w", err)
		}
	}

	// Build request
	reqURL := c.baseURL + "/" + endpoint

	// Add timestamp and sign
	params["openId"] = c.openID
	params["time"] = time.Now().Unix()

	// Generate signature
	sign, err := c.sign(params)
	if err != nil {
		return nil, fmt.Errorf("failed to sign request: %w", err)
	}
	params["sign"] = sign

	// Build form data
	formData := c.buildFormData(params)

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, strings.NewReader(formData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	c.logger.Debug("sending request", "endpoint", endpoint, "url", reqURL)

	// Send request with retry
	var resp *http.Response
	var lastErr error
	for i := 0; i <= 3; i++ {
		resp, err = c.httpClient.Do(req)
		if err == nil {
			break
		}
		lastErr = err
		if i < 3 {
			time.Sleep(time.Duration(i+1) * time.Second)
		}
	}
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed after retries: %w", lastErr)
	}
	defer resp.Body.Close()

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Parse response
	var result Response
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w (body: %s)", err, string(body))
	}

	// Check status
	if !result.Status {
		return nil, fmt.Errorf("API error: %s", result.Message)
	}

	// Cache the response
	if c.enableCache && cacheKey != "" && c.isCacheableRequest(endpoint, params) {
		c.cache.Set(cacheKey, &result)
	}

	return &result, nil
}

// sign generates the signature for the request parameters.
// QuickSDK signature algorithm:
// 1. Sort parameters by key name (natural order)
// 2. Concatenate as k1=v1&k2=v2&
// 3. Append openKey
// 4. Calculate MD5
func (c *Client) sign(params map[string]interface{}) (string, error) {
	// Sort keys
	keys := make([]string, 0, len(params))
	for k := range params {
		if k != "sign" { // Exclude sign from signature
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)

	// Build string
	var buf strings.Builder
	for _, k := range keys {
		buf.WriteString(k)
		buf.WriteString("=")
		buf.WriteString(fmt.Sprint(params[k]))
		buf.WriteString("&")
	}
	buf.WriteString(c.openKey)

	// Calculate MD5
	hash := md5.Sum([]byte(buf.String()))
	return fmt.Sprintf("%x", hash), nil
}

// buildFormData builds form-encoded data from parameters.
func (c *Client) buildFormData(params map[string]interface{}) string {
	var buf strings.Builder
	for k, v := range params {
		if buf.Len() > 0 {
			buf.WriteString("&")
		}
		buf.WriteString(url.QueryEscape(k))
		buf.WriteString("=")
		buf.WriteString(url.QueryEscape(fmt.Sprint(v)))
	}
	return buf.String()
}

// isCacheableRequest determines if a request is cacheable.
// GET-like requests (without side effects) are cacheable.
func (c *Client) isCacheableRequest(endpoint string, params map[string]interface{}) bool {
	// List/query operations are cacheable
	cacheableEndpoints := []string{
		"open/channelList",
		"open/serverList",
		"open/productList",
		"open/dayReport",
		"open/dayHourReport",
		"open/userLive",
		"open/channelDaysReport",
		"open/channelReport",
		"open/adReport",
		"open/getMediaApp",
		"open/getAdPlanGroup",
		"open/getPackageVersion",
		"open/getAdPages",
		"open/getAdPlan",
		"open/uwlLost",
	}

	for _, ce := range cacheableEndpoints {
		if endpoint == ce {
			return true
		}
	}
	return false
}

// buildCacheKey builds a cache key for a request.
func (c *Client) buildCacheKey(endpoint string, params map[string]interface{}) string {
	// Create a deterministic string from params
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var buf strings.Builder
	buf.WriteString(endpoint)
	for _, k := range keys {
		buf.WriteString(":")
		buf.WriteString(k)
		buf.WriteString("=")
		buf.WriteString(fmt.Sprint(params[k]))
	}
	return buf.String()
}

// Helper types for common parameter types

// Int64Value converts various int types to int64.
func Int64Value(v interface{}) int64 {
	switch val := v.(type) {
	case int:
		return int64(val)
	case int8:
		return int64(val)
	case int16:
		return int64(val)
	case int32:
		return int64(val)
	case int64:
		return val
	case uint:
		return int64(val)
	case uint8:
		return int64(val)
	case uint16:
		return int64(val)
	case uint32:
		return int64(val)
	case uint64:
		return int64(val)
	case float64:
		return int64(val)
	case string:
		i, _ := strconv.ParseInt(val, 10, 64)
		return i
	default:
		return 0
	}
}

// ParseTimestamp parses a timestamp to time.Time.
func ParseTimestamp(ts interface{}) time.Time {
	var sec int64
	switch t := ts.(type) {
	case int64:
		sec = t
	case float64:
		sec = int64(t)
	case string:
		sec, _ = strconv.ParseInt(t, 10, 64)
	}
	return time.Unix(sec, 0)
}

// rateLimiter is a simple token bucket rate limiter.
type rateLimiter struct {
	tokens   chan struct{}
	interval time.Duration
}

func newRateLimiter(requestsPerMinute int) *rateLimiter {
	if requestsPerMinute <= 0 {
		requestsPerMinute = 1000 // QuickSDK default
	}
	interval := time.Minute / time.Duration(requestsPerMinute)
	burstSize := max(1, requestsPerMinute/60) // Allow 1 second burst

	rl := &rateLimiter{
		tokens:   make(chan struct{}, burstSize),
		interval: interval,
	}

	// Fill initial tokens
	for i := 0; i < burstSize; i++ {
		rl.tokens <- struct{}{}
	}

	// Start refill goroutine
	go rl.refill()

	return rl
}

func (rl *rateLimiter) refill() {
	ticker := time.NewTicker(rl.interval)
	defer ticker.Stop()

	for range ticker.C {
		select {
		case rl.tokens <- struct{}{}:
		default:
			// Bucket is full, discard token
		}
	}
}

func (rl *rateLimiter) Wait(ctx context.Context) error {
	select {
	case <-rl.tokens:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// cache is a simple in-memory cache with TTL.
type cache struct {
	mu    sync.RWMutex
	items map[string]*cacheItem
	ttl   time.Duration
	done  chan struct{}
}

type cacheItem struct {
	value    *Response
	expireAt time.Time
}

func newCache(ttl time.Duration) *cache {
	c := &cache{
		items: make(map[string]*cacheItem),
		ttl:   ttl,
		done:  make(chan struct{}),
	}
	go c.cleanup()
	return c
}

func (c *cache) Set(key string, value *Response) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items[key] = &cacheItem{
		value:    value,
		expireAt: time.Now().Add(c.ttl),
	}
}

func (c *cache) Get(key string) (*Response, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	item, exists := c.items[key]
	if !exists {
		return nil, false
	}
	if time.Now().After(item.expireAt) {
		return nil, false
	}
	return item.value, true
}

func (c *cache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.items, key)
}

func (c *cache) Close() {
	close(c.done)
}

func (c *cache) cleanup() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.mu.Lock()
			now := time.Now()
			for k, item := range c.items {
				if now.After(item.expireAt) {
					delete(c.items, k)
				}
			}
			c.mu.Unlock()
		case <-c.done:
			return
		}
	}
}
