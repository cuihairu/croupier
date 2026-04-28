package agent

import (
	"context"
	"testing"
	"time"

	tcptr "github.com/cuihairu/croupier/internal/transport/tcp"
	sdkv1 "github.com/cuihairu/croupier/pkg/pb/croupier/sdk/v1"
	"github.com/cuihairu/croupier/pkg/protocol"
	"google.golang.org/protobuf/proto"
)

func TestTCPLocalListenerProviderSessionLifecycle(t *testing.T) {
	t.Parallel()

	store := NewProviderSessionStore()
	listener, err := NewTCPLocalListener(&TCPLocalListenerConfig{
		Address:     "127.0.0.1:0",
		RecvTimeout: 30 * time.Second,
		SendTimeout: 30 * time.Second,
	}, store, nil)
	if err != nil {
		t.Fatalf("new listener: %v", err)
	}
	defer listener.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- listener.Serve(ctx)
	}()

	client, err := tcptr.NewClient(&tcptr.Config{
		Address:        listener.Addr(),
		Insecure:       true,
		ConnectTimeout: 5 * time.Second,
		RecvTimeout:    30 * time.Second,
		SendTimeout:    30 * time.Second,
	})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	defer client.Close()

	connectReq := &sdkv1.ProviderConnectRequest{
		ServiceId:       "provider-1",
		Version:         "1.0.0",
		SdkLanguage:     "go",
		SdkVersion:      "test",
		Functions:       []*sdkv1.LocalFunctionDescriptor{{Id: "player.ban", Version: "1.0.0"}},
		ProtocolVersion: "v1",
	}
	connectBody, err := proto.Marshal(connectReq)
	if err != nil {
		t.Fatalf("marshal connect: %v", err)
	}

	respMsgID, respBody, err := client.Call(ctx, protocol.MsgProviderConnectRequest, connectBody)
	if err != nil {
		t.Fatalf("connect call: %v", err)
	}
	if respMsgID != protocol.MsgProviderConnectResponse {
		t.Fatalf("response msgID = %s, want %s", protocol.MsgIDString(respMsgID), protocol.MsgIDString(protocol.MsgProviderConnectResponse))
	}

	connectResp := &sdkv1.ProviderConnectResponse{}
	if err := proto.Unmarshal(respBody, connectResp); err != nil {
		t.Fatalf("unmarshal connect response: %v", err)
	}
	if connectResp.GetSessionId() == "" {
		t.Fatal("expected session id")
	}

	sess, ok := store.GetByServiceID("provider-1")
	if !ok {
		t.Fatal("provider session not stored")
	}
	initialLastSeen := sess.GetLastSeen()

	time.Sleep(1100 * time.Millisecond)

	hbReq := &sdkv1.ProviderHeartbeatRequest{
		ServiceId: "provider-1",
		SessionId: connectResp.GetSessionId(),
	}
	hbBody, err := proto.Marshal(hbReq)
	if err != nil {
		t.Fatalf("marshal heartbeat: %v", err)
	}
	respMsgID, _, err = client.Call(ctx, protocol.MsgProviderHeartbeatRequest, hbBody)
	if err != nil {
		t.Fatalf("heartbeat call: %v", err)
	}
	if respMsgID != protocol.MsgProviderHeartbeatResponse {
		t.Fatalf("heartbeat response msgID = %s, want %s", protocol.MsgIDString(respMsgID), protocol.MsgIDString(protocol.MsgProviderHeartbeatResponse))
	}

	sess, ok = store.GetBySessionID(connectResp.GetSessionId())
	if !ok {
		t.Fatal("provider session not found by session id")
	}
	if !sess.GetLastSeen().After(initialLastSeen) {
		t.Fatalf("heartbeat did not update last seen: before=%v after=%v", initialLastSeen, sess.GetLastSeen())
	}

	drainReq := &sdkv1.ProviderDrainRequest{
		SessionId:    connectResp.GetSessionId(),
		Reason:       "deploy",
		RetryAfterMs: 5000,
	}
	drainBody, err := proto.Marshal(drainReq)
	if err != nil {
		t.Fatalf("marshal drain: %v", err)
	}
	respMsgID, _, err = client.Call(ctx, protocol.MsgProviderDrainRequest, drainBody)
	if err != nil {
		t.Fatalf("drain call: %v", err)
	}
	if respMsgID != protocol.MsgProviderDrainResponse {
		t.Fatalf("drain response msgID = %s, want %s", protocol.MsgIDString(respMsgID), protocol.MsgIDString(protocol.MsgProviderDrainResponse))
	}

	if err := client.Close(); err != nil {
		t.Fatalf("close client: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, ok := store.GetBySessionID(connectResp.GetSessionId()); !ok {
			cancel()
			_ = listener.Close()
			<-done
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("provider session was not removed after disconnect")
}

func TestTCPLocalListenerRejectsNonProviderConnectFirstFrame(t *testing.T) {
	t.Parallel()

	store := NewProviderSessionStore()
	listener, err := NewTCPLocalListener(&TCPLocalListenerConfig{
		Address:     "127.0.0.1:0",
		RecvTimeout: 30 * time.Second,
		SendTimeout: 30 * time.Second,
	}, store, nil)
	if err != nil {
		t.Fatalf("new listener: %v", err)
	}
	defer listener.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- listener.Serve(ctx)
	}()

	client, err := tcptr.NewClient(&tcptr.Config{
		Address:        listener.Addr(),
		Insecure:       true,
		ConnectTimeout: 5 * time.Second,
		RecvTimeout:    30 * time.Second,
		SendTimeout:    30 * time.Second,
	})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	defer client.Close()

	_, _, err = client.Call(ctx, protocol.MsgProviderHeartbeatRequest, []byte("{}"))
	if err == nil {
		t.Fatal("expected connection close after invalid first frame")
	}

	if got := store.Count(); got != 0 {
		t.Fatalf("unexpected provider sessions stored: %d", got)
	}

	cancel()
	_ = listener.Close()
	<-done
}
