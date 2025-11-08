package jobs

import (
	"context"
	"net"
	"testing"
	"time"

	functionv1 "github.com/cuihairu/croupier/pkg/pb/croupier/function/v1"
	"google.golang.org/grpc"
)

// test function server to respond to Invoke
type fnServer struct {
	functionv1.UnimplementedFunctionServiceServer
}

func (s *fnServer) Invoke(ctx context.Context, req *functionv1.InvokeRequest) (*functionv1.InvokeResponse, error) {
	return &functionv1.InvokeResponse{Payload: []byte(`{"ok":true}`)}, nil
}

func startLocalFnServer(t *testing.T) (addr string, stop func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	s := grpc.NewServer()
	functionv1.RegisterFunctionServiceServer(s, &fnServer{})
	go s.Serve(ln)
	return ln.Addr().String(), func() { s.Stop(); _ = ln.Close() }
}

func TestExecutor_Start_IdempotencyAndCancel(t *testing.T) {
	addr, stop := startLocalFnServer(t)
	defer stop()

	e := NewExecutor()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	req := &functionv1.InvokeRequest{FunctionId: "x", IdempotencyKey: "same", Payload: []byte("{}")}
	job1, existed := e.Start(ctx, req, addr)
	if existed {
		t.Fatal("first start should not exist")
	}
	job2, existed2 := e.Start(ctx, req, addr)
	if !existed2 {
		t.Fatal("second start should exist")
	}
	if job1 != job2 {
		t.Fatalf("expect same job id for same idempotency: %s vs %s", job1, job2)
	}

	ch, ok := e.Stream(job1)
	if !ok {
		t.Fatal("stream not found")
	}
	// cancel and expect an error event; allow small delay for executor to set cancel func
	deadline := time.Now().Add(1 * time.Second)
	for {
		if e.Cancel(job1) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("cancel failed")
		}
		time.Sleep(10 * time.Millisecond)
	}
	// Do not require error event; ensure channel eventually closes
	<-ch
}
