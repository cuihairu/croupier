package agent

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	agentlocal "github.com/cuihairu/croupier/internal/platform/agentlocal"
	"github.com/cuihairu/croupier/internal/platform/tlsutil"
	functionv1 "github.com/cuihairu/croupier/pkg/pb/croupier/function/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

// FunctionServer forwards protobuf calls to local game servers that expose FunctionService.
type FunctionServer struct {
	functionv1.UnimplementedFunctionServiceServer
	store *agentlocal.LocalStore
	jobs  *jobIndex
}

// pickInstance returns the first available instance for a function id.
// Returns (addr, nil) on success, ("", error) on failure with appropriate gRPC status.
func (s *FunctionServer) pickInstance(fid string) (string, error) {
	if s.store == nil {
		return "", status.Error(codes.Internal, "instance store not initialized")
	}
	if fid == "" {
		return "", status.Error(codes.InvalidArgument, "function ID is required")
	}

	snap := s.store.List()
	arr := snap[fid]
	if len(arr) == 0 {
		// Check if the function ID exists in the registry at all
		hasFunction := false
		for functionID := range snap {
			if functionID == fid {
				hasFunction = true
				break
			}
		}

		if hasFunction {
			// Function exists but no instances are registered
			return "", status.Error(codes.Unavailable,
				fmt.Sprintf("function '%s' is registered but no instances are currently available", fid))
		} else {
			// Function is not registered at all
			return "", status.Error(codes.NotFound,
				fmt.Sprintf("function '%s' is not registered", fid))
		}
	}

	// Check if instances are healthy (not too old)
	now := time.Now()
	for _, inst := range arr {
		// Consider instance unhealthy if not seen for more than 30 seconds
		if now.Sub(inst.LastSeen) < 30*time.Second {
			return inst.Addr, nil
		}
	}

	// All instances are stale but return the first one with a warning
	return arr[0].Addr, status.Error(codes.Unavailable,
		fmt.Sprintf("function '%s' instances are stale (last seen > 30s ago)", fid))
}

func (s *FunctionServer) dial(addr string) (*grpc.ClientConn, functionv1.FunctionServiceClient, error) {
	if addr == "" {
		return nil, nil, status.Error(codes.Internal, "empty address provided")
	}

	var dialOpts []grpc.DialOption

	// Check if TLS is enabled for outbound connections
	tlsEnabled := os.Getenv("CROUPIER_OUTBOUND_TLS_ENABLED") == "true"
	if tlsEnabled {
		// Use TLS for outbound connections
		certFile := os.Getenv("CROUPIER_CLIENT_CERT_FILE")
		keyFile := os.Getenv("CROUPIER_CLIENT_KEY_FILE")
		caFile := os.Getenv("CROUPIER_CA_FILE")
		serverName := os.Getenv("CROUPIER_SERVER_NAME")

		creds, err := tlsutil.ClientTLS(certFile, keyFile, caFile, serverName)
		if err != nil {
			return nil, nil, status.Errorf(codes.Internal, "failed to create TLS credentials: %v", err)
		}
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(creds))
	} else {
		// Use insecure connection
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	// Add common options with timeout
	dialOpts = append(dialOpts,
		grpc.WithDefaultCallOptions(grpc.CallContentSubtype("json")),
	)

	// Add dial timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cc, err := grpc.DialContext(ctx, addr, dialOpts...)
	if err != nil {
		// Provide more specific error information
		if strings.Contains(err.Error(), "connection refused") {
			return nil, nil, status.Errorf(codes.Unavailable,
				"function instance at %s is not running (connection refused)", addr)
		} else if strings.Contains(err.Error(), "timeout") {
			return nil, nil, status.Errorf(codes.DeadlineExceeded,
				"timeout connecting to function instance at %s", addr)
		} else if strings.Contains(err.Error(), "no such host") {
			return nil, nil, status.Errorf(codes.Unavailable,
				"cannot resolve address %s", addr)
		}
		return nil, nil, status.Errorf(codes.Unavailable,
			"failed to connect to function instance at %s: %v", addr, err)
	}
	return cc, functionv1.NewFunctionServiceClient(cc), nil
}

func (s *FunctionServer) Invoke(ctx context.Context, in *functionv1.InvokeRequest) (*functionv1.InvokeResponse, error) {
	addr, err := s.pickInstance(in.GetFunctionId())
	if err != nil {
		// Error already has proper gRPC status
		return nil, err
	}
	cc, cli, err := s.dial(addr)
	if err != nil {
		return nil, status.Errorf(codes.Unavailable, "failed to connect to function instance at %s: %v", addr, err)
	}
	defer cc.Close()
	c2, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	return cli.Invoke(c2, in)
}

func (s *FunctionServer) StartJob(ctx context.Context, in *functionv1.InvokeRequest) (*functionv1.StartJobResponse, error) {
	addr, err := s.pickInstance(in.GetFunctionId())
	if err != nil {
		return nil, err
	}
	cc, cli, err := s.dial(addr)
	if err != nil {
		return nil, status.Errorf(codes.Unavailable, "failed to connect to function instance at %s: %v", addr, err)
	}
	defer cc.Close()
	c2, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	resp, err := cli.StartJob(c2, in)
	if err == nil && resp != nil && resp.GetJobId() != "" && s.jobs != nil {
		s.jobs.Set(resp.GetJobId(), addr)
	}
	return resp, err
}

func (s *FunctionServer) CancelJob(ctx context.Context, in *functionv1.CancelJobRequest) (*functionv1.StartJobResponse, error) {
	if in == nil || in.GetJobId() == "" {
		return nil, status.Error(codes.InvalidArgument, "job ID is required")
	}
	if s.jobs == nil {
		return nil, status.Error(codes.Internal, "job tracking not available")
	}
	if addr, ok := s.jobs.Get(in.GetJobId()); ok {
		cc, cli, err := s.dial(addr)
		if err == nil {
			defer cc.Close()
			c2, cancel := context.WithTimeout(ctx, 3*time.Second)
			defer cancel()
			resp, err2 := cli.CancelJob(c2, in)
			// best-effort: remove mapping after cancel
			s.jobs.Delete(in.GetJobId())
			if err2 == nil {
				return resp, nil
			}
			return nil, err2
		}
		return nil, status.Error(codes.Unavailable, "failed to connect to function instance")
	}
	// job not found in tracking
	return nil, status.Error(codes.NotFound, "job not found or already completed")
}

// StreamJob implements streaming for long-running jobs
func (s *FunctionServer) StreamJob(in *functionv1.JobStreamRequest, stream functionv1.FunctionService_StreamJobServer) error {
	if in == nil || in.GetJobId() == "" {
		return status.Error(codes.InvalidArgument, "job ID is required")
	}
	if s.jobs == nil {
		return status.Error(codes.Internal, "job tracking not available")
	}

	jobID := in.GetJobId()

	// Find job address
	addr, ok := s.jobs.Get(jobID)
	if !ok {
		return status.Error(codes.NotFound, "job not found or already completed")
	}

	// Connect to function instance
	cc, cli, err := s.dial(addr)
	if err != nil {
		return status.Errorf(codes.Unavailable, "failed to connect to function instance: %v", err)
	}
	defer cc.Close()

	// Use stream context (respects client-side cancellation and deadlines)
	ctx := stream.Context()

	// Create client stream
	clientStream, err := cli.StreamJob(ctx, in)
	if err != nil {
		// Check if job is not found
		if status.Code(err) == codes.NotFound {
			s.jobs.Delete(jobID)
		}
		return status.Errorf(codes.Internal, "failed to start job stream: %v", err)
	}

	// Forward events from client stream to server stream
	for {
		event, err := clientStream.Recv()
		if err != nil {
			// Handle stream completion or error
			if err == io.EOF || status.Code(err) == codes.OK {
				// Stream completed normally
				s.jobs.Delete(jobID)
				return nil
			}
			// Check if it's a gRPC status error
			if st, ok := status.FromError(err); ok {
				// For terminal states, clean up job tracking
				if isTerminalErrorCode(st.Code()) {
					s.jobs.Delete(jobID)
				}
				return st.Err()
			}
			// Unknown error
			s.jobs.Delete(jobID)
			return status.Errorf(codes.Internal, "job stream error: %v", err)
		}

		// Check if event indicates job completion
		if isTerminalEvent(event) {
			s.jobs.Delete(jobID)
		}

		// Forward event to client
		if err := stream.Send(event); err != nil {
			// Client disconnected
			s.jobs.Delete(jobID)
			return status.Error(codes.Canceled, "client disconnected")
		}
	}
}

// isTerminalEvent checks if the event type indicates job completion
func isTerminalEvent(evt *functionv1.JobEvent) bool {
	if evt == nil || evt.GetType() == "" {
		return false
	}
	switch strings.ToLower(evt.GetType()) {
	case "done", "completed", "error", "failed", "cancelled", "canceled", "succeeded", "success":
		return true
	default:
		return false
	}
}

// isTerminalErrorCode checks if the gRPC error code indicates a terminal state
func isTerminalErrorCode(code codes.Code) bool {
	switch code {
	case codes.OK, codes.Canceled, codes.Aborted, codes.OutOfRange, codes.NotFound, codes.AlreadyExists:
		return true
	default:
		return false
	}
}
