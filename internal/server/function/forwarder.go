package function

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/cuihairu/croupier/internal/transport/interceptors"
	jobv1 "github.com/cuihairu/croupier/pkg/pb/croupier/edge/job/v1"
	functionv1 "github.com/cuihairu/croupier/pkg/pb/croupier/function/v1"
	"google.golang.org/grpc"
)

// Forwarder forwards FunctionService calls to a remote endpoint (e.g., Edge),
// without local registry. It preserves request metadata (trace_id/game_id/env).
type Forwarder struct {
	functionv1.UnimplementedFunctionServiceServer
	addr string
}

func NewForwarder(addr string) *Forwarder { return &Forwarder{addr: addr} }

func (f *Forwarder) dial() (*grpc.ClientConn, error) {
	base := []grpc.DialOption{grpc.WithInsecure(), grpc.WithDefaultCallOptions(grpc.CallContentSubtype("json"))}
	opts := append(base, interceptors.Chain(nil)...)
	return grpc.Dial(f.addr, opts...)
}

func (f *Forwarder) Invoke(ctx context.Context, req *functionv1.InvokeRequest) (*functionv1.InvokeResponse, error) {
	slog.Info("forward invoke", "function_id", req.GetFunctionId(), "edge_addr", f.addr, "idempotency_key", req.GetIdempotencyKey())
	cc, err := f.dial()
	if err != nil {
		return nil, fmt.Errorf("dial edge: %w", err)
	}
	defer cc.Close()
	cli := functionv1.NewFunctionServiceClient(cc)
	return cli.Invoke(ctx, req)
}

func (f *Forwarder) StartJob(ctx context.Context, req *functionv1.InvokeRequest) (*functionv1.StartJobResponse, error) {
	slog.Info("forward start_job", "function_id", req.GetFunctionId(), "edge_addr", f.addr, "idempotency_key", req.GetIdempotencyKey())
	cc, err := f.dial()
	if err != nil {
		return nil, fmt.Errorf("dial edge: %w", err)
	}
	defer cc.Close()
	cli := functionv1.NewFunctionServiceClient(cc)
	return cli.StartJob(ctx, req)
}

func (f *Forwarder) StreamJob(req *functionv1.JobStreamRequest, srv functionv1.FunctionService_StreamJobServer) error {
	cc, err := f.dial()
	if err != nil {
		return fmt.Errorf("dial edge: %w", err)
	}
	defer cc.Close()
	cli := functionv1.NewFunctionServiceClient(cc)
	stream, err := cli.StreamJob(srv.Context(), req)
	if err != nil {
		return err
	}
	for {
		ev, err := stream.Recv()
		if err != nil {
			return err
		}
		if err := srv.Send(ev); err != nil {
			return err
		}
		if ev.GetType() == "done" || ev.GetType() == "error" {
			return nil
		}
	}
}

func (f *Forwarder) CancelJob(ctx context.Context, req *functionv1.CancelJobRequest) (*functionv1.StartJobResponse, error) {
	cc, err := f.dial()
	if err != nil {
		return nil, fmt.Errorf("dial edge: %w", err)
	}
	defer cc.Close()
	cli := functionv1.NewFunctionServiceClient(cc)
	return cli.CancelJob(ctx, req)
}

// ForwarderInvoker implements an invoker interface for HTTP server by dialing Edge.
type ForwarderInvoker struct{ f *Forwarder }

func NewForwarderInvoker(f *Forwarder) *ForwarderInvoker { return &ForwarderInvoker{f: f} }

func (i *ForwarderInvoker) Invoke(ctx context.Context, req *functionv1.InvokeRequest) (*functionv1.InvokeResponse, error) {
	return i.f.Invoke(ctx, req)
}
func (i *ForwarderInvoker) StartJob(ctx context.Context, req *functionv1.InvokeRequest) (*functionv1.StartJobResponse, error) {
	return i.f.StartJob(ctx, req)
}
func (i *ForwarderInvoker) CancelJob(ctx context.Context, req *functionv1.CancelJobRequest) (*functionv1.StartJobResponse, error) {
	return i.f.CancelJob(ctx, req)
}
func (i *ForwarderInvoker) StreamJob(ctx context.Context, req *functionv1.JobStreamRequest) (functionv1.FunctionService_StreamJobClient, error) {
	cc, err := i.f.dial()
	if err != nil {
		return nil, fmt.Errorf("dial edge: %w", err)
	}
	cli := functionv1.NewFunctionServiceClient(cc)
	// NOTE: cc leaked for stream lifetime in PoC
	return cli.StreamJob(ctx, req)
}

// Optional: job result fetch via Edge JobService (used by HTTP /api/job_result when Server is in edge-forward mode)
func (i *ForwarderInvoker) JobResult(ctx context.Context, jobID string) (state string, payload []byte, errMsg string, err error) {
	cc, err2 := i.f.dial()
	if err2 != nil {
		return "", nil, "", fmt.Errorf("dial edge: %w", err2)
	}
	cli := jobv1.NewJobServiceClient(cc)
	resp, err2 := cli.GetJobResult(ctx, &jobv1.GetJobResultRequest{JobId: jobID})
	if err2 != nil {
		return "", nil, "", err2
	}
	return resp.State, resp.Payload, resp.Error, nil
}
