// Copyright 2025 Croupier Authors
// Licensed under the Apache License, Version 2.0

package transport

import (
	"testing"
	"time"
)

func TestConfigValidation_DefaultValues(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Address != "127.0.0.1:19091" {
		t.Errorf("expected Address 127.0.0.1:19091, got %s", cfg.Address)
	}
	if cfg.Host != "127.0.0.1" {
		t.Errorf("expected Host 127.0.0.1, got %s", cfg.Host)
	}
	if cfg.Port != 19091 {
		t.Errorf("expected Port 19091, got %d", cfg.Port)
	}
	if !cfg.Insecure {
		t.Error("expected Insecure true")
	}
	if cfg.DialTimeout != 30*time.Second {
		t.Errorf("expected DialTimeout 30s, got %v", cfg.DialTimeout)
	}
	if cfg.RecvTimeout != 30*time.Second {
		t.Errorf("expected RecvTimeout 30s, got %v", cfg.RecvTimeout)
	}
	if cfg.SendTimeout != 5*time.Second {
		t.Errorf("expected SendTimeout 5s, got %v", cfg.SendTimeout)
	}
	if cfg.ReadQLen != 128 {
		t.Errorf("expected ReadQLen 128, got %d", cfg.ReadQLen)
	}
	if cfg.WriteQLen != 64 {
		t.Errorf("expected WriteQLen 64, got %d", cfg.WriteQLen)
	}
}

func TestCreateTLSConfig_Insecure(t *testing.T) {
	cfg := &Config{Insecure: true}

	tlsCfg, err := createTLSConfig(cfg)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if tlsCfg != nil {
		t.Error("expected nil TLS config for insecure mode")
	}
}

func TestCreateTLSConfig_SecureWithoutCA(t *testing.T) {
	cfg := &Config{Insecure: false}

	tlsCfg, err := createTLSConfig(cfg)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if tlsCfg == nil {
		t.Fatal("expected TLS config")
	}
}

func TestCreateTLSConfig_InvalidCAFile(t *testing.T) {
	cfg := &Config{
		Insecure: false,
		CAFile:   "/nonexistent/ca.pem",
	}

	_, err := createTLSConfig(cfg)
	if err == nil {
		t.Error("expected error for nonexistent CA file")
	}
}

func TestCreateTLSConfig_WithServerName(t *testing.T) {
	cfg := &Config{
		Insecure:   false,
		ServerName: "example.com",
	}

	tlsCfg, err := createTLSConfig(cfg)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if tlsCfg.ServerName != "example.com" {
		t.Errorf("expected ServerName example.com, got %s", tlsCfg.ServerName)
	}
}

func TestCreateTLSConfig_WithCertKeyMissing(t *testing.T) {
	cfg := &Config{
		Insecure: false,
		CertFile: "/nonexistent/cert.pem",
		KeyFile:  "/nonexistent/key.pem",
	}

	_, err := createTLSConfig(cfg)
	if err == nil {
		t.Error("expected error for nonexistent cert/key files")
	}
}

func TestDialAddr(t *testing.T) {
	tests := []struct {
		name     string
		cfg      *Config
		expected string
	}{
		{
			name:     "default config",
			cfg:      DefaultConfig(),
			expected: "tcp://127.0.0.1:19091",
		},
		{
			name: "custom address",
			cfg: &Config{
				Address:  "192.168.1.1:8080",
				Insecure: true,
			},
			expected: "tcp://192.168.1.1:8080",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := dialAddr(tt.cfg)
			if result != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestDialAddr_EmptyAddresses(t *testing.T) {
	cfg := &Config{}

	result := dialAddr(cfg)
	if result != "tcp://127.0.0.1:19091" {
		t.Errorf("expected default address, got %s", result)
	}
}
