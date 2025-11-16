package control

import (
    "bytes"
    "compress/gzip"
    "context"
    "encoding/json"
    "fmt"
    "io"
    "time"

    reg "github.com/cuihairu/croupier/internal/platform/registry"
    controlv1 "github.com/cuihairu/croupier/pkg/pb/croupier/control/v1"
    commonv1 "github.com/cuihairu/croupier/pkg/pb/croupier/common/v1"
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
    enabledMap := map[string]bool{}

    s.reg.Mu().RLock()
    for _, a := range s.reg.AgentsUnsafe() {
        if a == nil || a.Functions == nil {
            continue
        }
        for fid, meta := range a.Functions {
            if fid == "" {
                continue
            }
            enabledMap[fid] = enabledMap[fid] || meta.Enabled
            seen[fid] = struct{}{}
        }
    }
    // Build metadata index from provider manifests
    metaIdx := s.reg.BuildFunctionIndex()
    s.reg.Mu().RUnlock()

    // Union of ids from enabledMap and metaIdx
    union := map[string]struct{}{}
    for k := range enabledMap {
        union[k] = struct{}{}
    }
    for k := range metaIdx {
        union[k] = struct{}{}
    }
    // Build descriptors
    for fid := range union {
        fd := &controlv1.FunctionDescriptor{
            Id:      fid,
            Enabled: enabledMap[fid],
        }
        if m, ok := metaIdx[fid]; ok {
            // display_name / summary (I18nText)
            if dn := parseI18n(m["display_name"]); dn != nil {
                fd.DisplayName = dn
            }
            if sm := parseI18n(m["summary"]); sm != nil {
                fd.Summary = sm
            }
            // tags
            if tags := parseStringSlice(m["tags"]); len(tags) > 0 {
                fd.Tags = tags
            }
            // menu
            if menu := parseMenu(m["menu"]); menu != nil {
                fd.Menu = menu
            }
            // permissions
            if perm := parsePerm(m["permissions"]); perm != nil {
                fd.Permissions = perm
            }
        }
        out.Functions = append(out.Functions, fd)
    }

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

// --- helpers to parse provider manifest metadata into proto types ---

func parseI18n(v interface{}) *commonv1.I18nText {
    if v == nil {
        return nil
    }
    switch t := v.(type) {
    case map[string]interface{}:
        out := &commonv1.I18nText{}
        if en, ok := t["en"].(string); ok {
            out.En = en
        }
        if zh, ok := t["zh"].(string); ok {
            out.Zh = zh
        }
        if out.En == "" && out.Zh == "" {
            return nil
        }
        return out
    case string:
        // Shortcut: string treated as zh
        return &commonv1.I18nText{Zh: t}
    default:
        // Try JSON string
        if s, ok := v.(string); ok {
            var m map[string]string
            if err := json.Unmarshal([]byte(s), &m); err == nil {
                out := &commonv1.I18nText{En: m["en"], Zh: m["zh"]}
                if out.En != "" || out.Zh != "" {
                    return out
                }
            }
        }
    }
    return nil
}

func parseStringSlice(v interface{}) []string {
    if v == nil {
        return nil
    }
    out := []string{}
    switch t := v.(type) {
    case []interface{}:
        for _, it := range t {
            if s, ok := it.(string); ok && s != "" {
                out = append(out, s)
            }
        }
    case []string:
        out = append(out, t...)
    }
    return out
}

func parseMenu(v interface{}) *commonv1.Menu {
    m, ok := v.(map[string]interface{})
    if !ok {
        return nil
    }
    out := &commonv1.Menu{}
    if s, ok := m["section"].(string); ok {
        out.Section = s
    }
    if s, ok := m["group"].(string); ok {
        out.Group = s
    }
    if s, ok := m["path"].(string); ok {
        out.Path = s
    }
    if f, ok := toFloat(m["order"]); ok {
        out.Order = int32(f)
    }
    if s, ok := m["icon"].(string); ok {
        out.Icon = s
    }
    if s, ok := m["badge"].(string); ok {
        out.Badge = s
    }
    if b, ok := m["hidden"].(bool); ok {
        out.Hidden = b
    }
    return out
}

func parsePerm(v interface{}) *commonv1.PermissionSpec {
    m, ok := v.(map[string]interface{})
    if !ok {
        return nil
    }
    out := &commonv1.PermissionSpec{}
    if verbs := parseStringSlice(m["verbs"]); len(verbs) > 0 {
        out.Verbs = verbs
    }
    if scopes := parseStringSlice(m["scopes"]); len(scopes) > 0 {
        out.Scopes = scopes
    }
    // defaults
    if defs, ok := m["defaults"].([]interface{}); ok {
        for _, d := range defs {
            dm, ok := d.(map[string]interface{})
            if !ok {
                continue
            }
            rb := &commonv1.RoleBinding{}
            if r, ok := dm["role"].(string); ok {
                rb.Role = r
            }
            if vs := parseStringSlice(dm["verbs"]); len(vs) > 0 {
                rb.Verbs = vs
            }
            if rb.Role != "" && len(rb.Verbs) > 0 {
                out.Defaults = append(out.Defaults, rb)
            }
        }
    }
    if zhMap, ok := m["i18n_zh"].(map[string]interface{}); ok {
        out.I18NZh = map[string]string{}
        for k, vv := range zhMap {
            if s, ok := vv.(string); ok {
                out.I18NZh[k] = s
            }
        }
    }
    return out
}

func toFloat(v interface{}) (float64, bool) {
    switch t := v.(type) {
    case float64:
        return t, true
    case int:
        return float64(t), true
    case int32:
        return float64(t), true
    case int64:
        return float64(t), true
    }
    return 0, false
}
