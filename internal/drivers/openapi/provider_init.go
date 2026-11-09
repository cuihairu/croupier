package openapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/cuihairu/croupier/internal/platform/provider"
	"github.com/cuihairu/croupier/internal/platform/ratelimit"
)

// Name returns the provider name.
func (p *Provider) Name() string {
	return "openapi"
}

func (p *Provider) Kind() string {
	return "openapi"
}

// Init initializes the provider with configuration.
func (p *Provider) Init(ctx context.Context, config provider.ProviderConfig) error {
	p.config = config
	p.openapiConfig = &Config{}

	// Extract OpenAPI config from provider config
	if err := p.extractConfig(config.Config); err != nil {
		return fmt.Errorf("failed to extract config: %w", err)
	}

	// Initialize HTTP client
	timeout := p.openapiConfig.Timeout.Duration()
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	p.httpClient = &http.Client{
		Timeout: timeout,
	}

	// Initialize rate limiter if configured
	if config.RateLimit != nil {
		p.rateLimiter = ratelimit.NewTokenBucket(
			config.RateLimit.RequestsPerMinute,
			config.RateLimit.BurstSize,
		)
	}

	// Build method map
	p.buildMethodMap()

	// Auto-discover methods if OpenAPI spec(s) are provided
	if len(p.openapiConfig.Methods) == 0 {
		// Use multiple specs if available
		if len(p.openapiConfig.OpenAPISpecs) > 0 {
			for _, specURL := range p.openapiConfig.OpenAPISpecs {
				if err := p.discoverMethodsFromSpec(ctx, specURL); err != nil {
					return fmt.Errorf("failed to discover methods from %s: %w", specURL, err)
				}
			}
		} else if p.openapiConfig.OpenAPISpec != "" {
			// Fallback to single spec for backward compatibility
			if err := p.discoverMethodsFromSpec(ctx, p.openapiConfig.OpenAPISpec); err != nil {
				return fmt.Errorf("failed to discover methods: %w", err)
			}
		}
	}

	return nil
}

// extractConfig extracts OpenAPI config from the generic provider config.
func (p *Provider) extractConfig(config map[string]interface{}) error {
	// Marshal and unmarshal to convert map to struct
	data, err := json.Marshal(config)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, p.openapiConfig)
}

// buildMethodMap builds the method name to API definition mapping.
// 仅遍历配置切片构建索引，无任何出错路径——原 error 返回值恒为 nil，
// 属"签名声明 error 但实现无出错路径"的设计债，签名已收紧。
func (p *Provider) buildMethodMap() {
	p.methods = make([]string, 0, len(p.openapiConfig.Methods))
	p.methodMap = make(map[string]*APIMethod)

	for i := range p.openapiConfig.Methods {
		method := &p.openapiConfig.Methods[i]
		if method.Name == "" {
			continue
		}
		p.methodMap[method.Name] = method
		p.methods = append(p.methods, method.Name)
	}
}
