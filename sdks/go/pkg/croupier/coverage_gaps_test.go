// Copyright 2025 Croupier Authors
// Licensed under the Apache License, Version 2.0

package croupier

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	sdkv1 "github.com/cuihairu/croupier/sdks/go/pkg/pb/croupier/sdk/v1"
)

// TestTCPInvokerConnectRacesWithStateFlip exercises the post-write-lock
// double-check branches in tcpInvoker.connect (connected / isReconnecting)
// by racing connect against a goroutine that flips the connection state
// under the write lock: the "idle" state is held only briefly while the
// flipped state is held long, so callers that pass the read-lock checks
// during the idle window reliably acquire the write lock afterwards and hit
// the double-check path.
func TestTCPInvokerConnectRacesWithStateFlip(t *testing.T) {
	flipRace := func(t *testing.T, flip func(inv *tcpInvoker), hold time.Duration) {
		t.Helper()
		inv := newTCPInvoker(&InvokerConfig{
			Address:   "127.0.0.1:1",
			Insecure:  true,
			Reconnect: DefaultReconnectConfig(),
			Retry:     DefaultRetryConfig(),
		}).(*tcpInvoker)

		stop := make(chan struct{})
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					inv.mu.Lock()
					flip(inv)
					time.Sleep(hold)
					inv.mu.Unlock()

					inv.mu.Lock()
					inv.connected = false
					inv.isReconnecting = false
					time.Sleep(20 * time.Microsecond)
					inv.mu.Unlock()
				}
			}
		}()

		for i := 0; i < 800; i++ {
			_ = inv.connect(context.Background())
		}
		close(stop)
		wg.Wait()

		_ = inv.Close()
	}

	t.Run("double-check connected", func(t *testing.T) {
		flipRace(t, func(inv *tcpInvoker) {
			inv.connected = true
			inv.isReconnecting = false
		}, 400*time.Microsecond)
	})

	t.Run("double-check reconnecting", func(t *testing.T) {
		flipRace(t, func(inv *tcpInvoker) {
			inv.connected = false
			inv.isReconnecting = true
		}, 400*time.Microsecond)
	})
}

// TestMaybeRegisterCapabilitiesInvalidManifest covers the manifest marshal
// failure branch: a descriptor whose input schema is not valid JSON makes
// json.Marshal(json.RawMessage) fail and the capability upload aborts before
// touching the control plane.
func TestMaybeRegisterCapabilitiesInvalidManifest(t *testing.T) {
	m := &TCPManager{config: ClientConfig{ControlAddr: "127.0.0.1:1"}}
	funcs := []*sdkv1.ProviderFunctionDescriptor{
		{Id: "bad-schema", Version: "1.0.0", InputSchema: `{"type":`},
	}
	m.maybeRegisterCapabilities("svc", "1.0.0", funcs)
}

// TestRPCHandlerInvokeRejectedWhileDraining covers the draining guard in
// tcpRPCHandler.invoke: while draining, new invokes receive an error payload
// instead of reaching the handler.
func TestRPCHandlerInvokeRejectedWhileDraining(t *testing.T) {
	m := &TCPManager{}
	m.draining.Store(true)
	h := newTCPRPCHandler(m)

	resp, err := h.invoke(context.Background(), 0, 0, nil)
	if err != nil {
		t.Fatalf("invoke during drain: %v", err)
	}
	if !strings.Contains(string(resp), "provider is draining") {
		t.Fatalf("unexpected drain payload: %q", resp)
	}
}

// TestHandleFilePushStagingWriteFailure covers the atomic write failure
// branch in handleFilePushRequest: the target basename is pre-created as a
// directory so the tmp-file rename fails (EISDIR) and the response reports
// the staging write error.
func TestHandleFilePushStagingWriteFailure(t *testing.T) {
	dir := t.TempDir()
	m := &TCPManager{config: ClientConfig{
		EnableFileTransfer: true,
		FileStagingDir:     dir,
		MaxFileSize:        1 << 20,
	}}
	if err := os.Mkdir(filepath.Join(dir, "blob.bin"), 0o750); err != nil {
		t.Fatalf("pre-create target dir: %v", err)
	}

	data := []byte("payload")
	sum := sha256.Sum256(data)
	body := encodeFilePushRequest(&filePushRequest{
		transferID:    "t-write-fail",
		fileName:      "blob.bin",
		contentSha256: hex.EncodeToString(sum[:]),
		data:          data,
	})

	resp, err := m.handleFilePushRequest(body)
	if err != nil {
		t.Fatalf("handleFilePushRequest: %v", err)
	}
	if !strings.Contains(string(resp), "write staging file") {
		t.Fatalf("expected write failure detail in response: %x", resp)
	}
}
