package openapi

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cuihairu/croupier/internal/drivers"
	"github.com/cuihairu/croupier/internal/platform/provider"
)

func init() {
	drivers.RegisterContext("openapi", newDriver)
}

func newDriver(ctx context.Context, cfg drivers.Config) (drivers.Driver, error) {
	config := map[string]any{}
	for key, value := range cfg.Raw {
		config[key] = value
	}
	for key, raw := range cfg.Extra {
		var value any
		if err := json.Unmarshal([]byte(raw), &value); err != nil {
			config[key] = raw
		} else {
			config[key] = value
		}
	}
	if cfg.Endpoint != "" {
		config["base_url"] = cfg.Endpoint
	}
	if cfg.Token != "" {
		if _, exists := config["auth"]; !exists {
			config["auth"] = map[string]any{"type": "bearer", "token": cfg.Token}
		}
	}

	instance := NewProvider()
	if err := instance.Init(ctx, provider.ProviderConfig{
		Enabled: true,
		Type:    "openapi",
		Config:  config,
	}); err != nil {
		return nil, fmt.Errorf("openapi driver init: %w", err)
	}
	return instance, nil
}
