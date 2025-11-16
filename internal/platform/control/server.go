package control

import (
    "bytes"
    "compress/gzip"
    "context"
    "fmt"
    "io"
    "time"

    reg "github.com/cuihairu/croupier/internal/platform/registry"
    controlv1 "github.com/cuihairu/croupier/pkg/pb/croupier/control/v1"
    "google.golang.org/protobuf/types/known/emptypb"
)

// Server implements the ControlService and exposes a registry store for other components.
type Server struct {
    controlv1.UnimplementedControlServiceServer
    reg *reg.Store
}

func NewServer(registry *reg.Store) *Server {
    if registry == nil {
        registry = reg.NewStore()
    }
    return &Server{reg: registry}
}

// Store returns the underlying registry Store (for function server / HTTP handlers).
func (s *Server) Store() *reg.Store { return s.reg }

// Register registers or updates an agent session. Minimal fields are accepted.
func (s *Server) Register(ctx context.Context, in *controlv1.RegisterRequest) (*controlv1.RegisterResponse, error) {
    if in == nil { return &controlv1.RegisterResponse{}, nil }
    sess := &reg.AgentSession{
        AgentID:  in.GetAgentId(),
        GameID:   in.GetGameId(),
        Env:      in.GetEnv(),
        RPCAddr:  in.GetRpcAddr(),
        Version:  in.GetVersion(),
        // Region/Zone/Labels are not present in current proto; leave empty
        ExpireAt: time.Now().Add(60 * time.Second),
        Functions: map[string]reg.FunctionMeta{},
    }
    // Populate functions from request descriptors (id -> enabled)
    if in.Functions != nil {
        for _, f := range in.Functions {
            if f == nil || f.GetId() == "" { continue }
            sess.Functions[f.GetId()] = reg.FunctionMeta{Enabled: f.GetEnabled()}
        }
    }
    s.reg.UpsertAgent(sess)
    return &controlv1.RegisterResponse{}, nil
}

// Heartbeat extends the expiry of an agent session.
func (s *Server) Heartbeat(ctx context.Context, in *controlv1.HeartbeatRequest) (*controlv1.HeartbeatResponse, error) {
    if in == nil || in.GetAgentId() == "" { return &controlv1.HeartbeatResponse{}, nil }
    s.reg.Mu().Lock()
    if a := s.reg.AgentsUnsafe()[in.GetAgentId()]; a != nil {
        a.ExpireAt = time.Now().Add(60 * time.Second)
    }
    s.reg.Mu().Unlock()
    return &controlv1.HeartbeatResponse{}, nil
}

// RegisterCapabilities handles provider manifest registration with language-agnostic declaration.
func (s *Server) RegisterCapabilities(ctx context.Context, in *controlv1.RegisterCapabilitiesRequest) (*controlv1.RegisterCapabilitiesResponse, error) {
    if in == nil {
        return &controlv1.RegisterCapabilitiesResponse{}, fmt.Errorf("request cannot be nil")
    }

    provider := in.GetProvider()
    if provider == nil || provider.GetId() == "" {
        return &controlv1.RegisterCapabilitiesResponse{}, fmt.Errorf("provider metadata is required")
    }

    // Decompress the manifest JSON
    manifestData, err := s.decompressManifest(in.GetManifestJsonGz())
    if err != nil {
        return &controlv1.RegisterCapabilitiesResponse{}, fmt.Errorf("failed to decompress manifest: %w", err)
    }

    // Store the provider capabilities in registry
    providerCaps := reg.ProviderCaps{
        ID:        provider.GetId(),
        Version:   provider.GetVersion(),
        Lang:      provider.GetLang(),
        SDK:       provider.GetSdk(),
        Manifest:  manifestData,
        UpdatedAt: time.Now(),
    }

    s.reg.UpsertProviderCaps(providerCaps)

    return &controlv1.RegisterCapabilitiesResponse{}, nil
}

// ListFunctionsSummary aggregates unique functions across all registered agents and returns
// a summarized descriptor list for dashboard consumption. This is a minimal baseline that
// can be enriched with UI/RBAC metadata sourced from proto options or provider manifests.
func (s *Server) ListFunctionsSummary(ctx context.Context, _ *emptypb.Empty) (*controlv1.ListFunctionsSummaryResponse, error) {
    out := &controlv1.ListFunctionsSummaryResponse{}
    seen := map[string]struct{}{}

    s.reg.Mu().RLock()
    for _, a := range s.reg.AgentsUnsafe() {
        if a == nil || a.Functions == nil {
            continue
        }
        for fid, meta := range a.Functions {
            if fid == "" {
                continue
            }
            if _, ok := seen[fid]; ok {
                continue
            }
            seen[fid] = struct{}{}
            // Fill minimal fields for now; UI/RBAC/i18n can be populated later
            fd := &controlv1.FunctionDescriptor{
                Id:      fid,
                Enabled: meta.Enabled,
            }
            out.Functions = append(out.Functions, fd)
        }
    }
    s.reg.Mu().RUnlock()

    return out, nil
}

// decompressManifest decompresses gzipped manifest data
func (s *Server) decompressManifest(data []byte) ([]byte, error) {
    if len(data) == 0 {
        return nil, fmt.Errorf("manifest data is empty")
    }

    reader, err := gzip.NewReader(bytes.NewReader(data))
    if err != nil {
        return nil, fmt.Errorf("failed to create gzip reader: %w", err)
    }
    defer reader.Close()

    return io.ReadAll(reader)
}
