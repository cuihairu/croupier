package control

import (
	"bytes"
	"compress/gzip"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	reg "github.com/cuihairu/croupier/internal/platform/registry"
	commonv1 "github.com/cuihairu/croupier/pkg/pb/croupier/common/v1"
	serverv1 "github.com/cuihairu/croupier/pkg/pb/croupier/server/v1"
	"github.com/xeipuuv/gojsonschema"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

//go:embed providers-manifest.schema.json
var embeddedProviderManifestSchema []byte

// Server implements the ControlService and exposes a registry store for other components.
type Server struct {
	serverv1.UnimplementedControlServiceServer
	reg          *reg.Store
	schemaLoader *gojsonschema.Schema // 缓存 Provider Manifest JSON Schema
	upstream     serverv1.ControlServiceClient
}

func NewServer(registry *reg.Store) *Server {
	if registry == nil {
		registry = reg.NewStore()
	}
	s := &Server{reg: registry}

	// 加载 Provider Manifest JSON Schema
	if err := s.loadProviderSchema(); err != nil {
		// Schema 加载失败不应该阻止服务启动，但应该记录日志
		// 这里使用 fmt.Printf 因为 logx 可能还未初始化；若需禁用可用 CROUPIER_PROVIDER_MANIFEST_SCHEMA=off
		fmt.Printf("Warning: Failed to load provider manifest schema: %v\n", err)
	}

	return s
}

// SetUpstreamClient configures an optional upstream ControlService client.
// When set (typically in Edge), Register/Heartbeat/Capabilities can be forwarded to the central Server.
func (s *Server) SetUpstreamClient(cli serverv1.ControlServiceClient) {
	s.upstream = cli
}

// loadProviderSchema 加载 Provider Manifest JSON Schema
func (s *Server) loadProviderSchema() error {
	if schemaPath := os.Getenv("CROUPIER_PROVIDER_MANIFEST_SCHEMA"); strings.TrimSpace(schemaPath) != "" {
		if strings.EqualFold(strings.TrimSpace(schemaPath), "off") {
			s.schemaLoader = nil
			return nil
		}
		schemaData, err := os.ReadFile(schemaPath)
		if err != nil {
			return fmt.Errorf("failed to read provider manifest schema from %q: %w", schemaPath, err)
		}
		schemaLoader, err := gojsonschema.NewSchema(gojsonschema.NewBytesLoader(schemaData))
		if err != nil {
			return fmt.Errorf("failed to parse provider manifest schema from %q: %w", schemaPath, err)
		}
		s.schemaLoader = schemaLoader
		return nil
	}

	// Dev fallback: allow overriding embedded schema with repo docs file when running from repo root.
	schemaData, err := os.ReadFile("docs/providers-manifest.schema.json")
	if err != nil {
		schemaData = embeddedProviderManifestSchema
	}

	schemaLoader, err := gojsonschema.NewSchema(gojsonschema.NewBytesLoader(schemaData))
	if err != nil {
		return fmt.Errorf("failed to parse provider manifest schema: %w", err)
	}

	s.schemaLoader = schemaLoader
	return nil
}

// Store returns the underlying registry Store (for function server / HTTP handlers).
func (s *Server) Store() *reg.Store { return s.reg }

// Register registers or updates an agent session. Minimal fields are accepted.
func (s *Server) Register(ctx context.Context, in *serverv1.RegisterRequest) (*serverv1.RegisterResponse, error) {
	if in == nil {
		return &serverv1.RegisterResponse{}, nil
	}

	var upstreamResp *serverv1.RegisterResponse
	if s.upstream != nil {
		resp, err := s.upstream.Register(ctx, in)
		if err != nil {
			return nil, err
		}
		upstreamResp = resp
	}

	sess := &reg.AgentSession{
		AgentID: in.GetAgentId(),
		GameID:  in.GetGameId(),
		Env:     in.GetEnv(),
		RPCAddr: in.GetRpcAddr(),
		Version: in.GetVersion(),
		// Region/Zone/Labels are not present in current proto; leave empty
		ExpireAt:  time.Now().Add(60 * time.Second),
		Functions: map[string]reg.FunctionMeta{},
	}
	// Populate functions from request descriptors (id -> enabled)
	if in.Functions != nil {
		for _, f := range in.Functions {
			if f == nil || f.GetId() == "" {
				continue
			}
			sess.Functions[f.GetId()] = reg.FunctionMeta{Enabled: f.GetEnabled(), Version: strings.TrimSpace(f.GetVersion())}
		}
	}

	if in.Processes != nil {
		processes := make([]reg.ProcessSession, 0, len(in.Processes))
		for _, p := range in.Processes {
			if p == nil || strings.TrimSpace(p.GetServiceId()) == "" {
				continue
			}
			functionIDs := make([]string, 0, len(p.GetFunctionIds()))
			for _, fid := range p.GetFunctionIds() {
				fid = strings.TrimSpace(fid)
				if fid != "" {
					functionIDs = append(functionIDs, fid)
				}
			}
			processes = append(processes, reg.ProcessSession{
				ServiceID:    strings.TrimSpace(p.GetServiceId()),
				Addr:         strings.TrimSpace(p.GetAddr()),
				Version:      strings.TrimSpace(p.GetVersion()),
				LastSeenUnix: p.GetLastSeenUnix(),
				FunctionIDs:  functionIDs,
			})
		}
		sess.Processes = processes
	}
	s.reg.UpsertAgent(sess)
	if upstreamResp != nil {
		return upstreamResp, nil
	}
	return &serverv1.RegisterResponse{}, nil
}

// Heartbeat extends the expiry of an agent session.
func (s *Server) Heartbeat(ctx context.Context, in *serverv1.HeartbeatRequest) (*serverv1.HeartbeatResponse, error) {
	if in == nil || in.GetAgentId() == "" {
		return &serverv1.HeartbeatResponse{}, nil
	}

	var upstreamResp *serverv1.HeartbeatResponse
	if s.upstream != nil {
		resp, err := s.upstream.Heartbeat(ctx, in)
		if err != nil {
			return nil, err
		}
		upstreamResp = resp
	}

	s.reg.Mu().Lock()
	if a := s.reg.AgentsUnsafe()[in.GetAgentId()]; a != nil {
		a.ExpireAt = time.Now().Add(60 * time.Second)
	}
	s.reg.Mu().Unlock()
	if upstreamResp != nil {
		return upstreamResp, nil
	}
	return &serverv1.HeartbeatResponse{}, nil
}

// RegisterCapabilities handles provider manifest registration with language-agnostic declaration.
func (s *Server) RegisterCapabilities(ctx context.Context, in *serverv1.RegisterCapabilitiesRequest) (*serverv1.RegisterCapabilitiesResponse, error) {
	if in == nil {
		return nil, status.Error(codes.InvalidArgument, "request cannot be nil")
	}

	provider := in.GetProvider()
	if provider == nil || provider.GetId() == "" {
		return nil, status.Error(codes.InvalidArgument, "provider metadata is required")
	}

	var upstreamResp *serverv1.RegisterCapabilitiesResponse
	if s.upstream != nil {
		resp, err := s.upstream.RegisterCapabilities(ctx, in)
		if err != nil {
			return nil, err
		}
		upstreamResp = resp
	}

	// Decompress the manifest JSON
	manifestData, err := s.decompressManifest(in.GetManifestJsonGz())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid manifest (gzip): %v", err)
	}

	// Validate manifest against JSON Schema if schema is available
	if s.schemaLoader != nil {
		if err := s.validateManifest(manifestData); err != nil {
			return nil, err
		}
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

	if upstreamResp != nil {
		return upstreamResp, nil
	}
	return &serverv1.RegisterCapabilitiesResponse{}, nil
}

// ListFunctionsSummary aggregates unique functions across all registered agents and returns
// a summarized descriptor list for dashboard consumption. This is a minimal baseline that
// can be enriched with UI/RBAC metadata sourced from proto options or provider manifests.
func (s *Server) ListFunctionsSummary(ctx context.Context, _ *emptypb.Empty) (*serverv1.ListFunctionsSummaryResponse, error) {
	if s.upstream != nil {
		return s.upstream.ListFunctionsSummary(ctx, &emptypb.Empty{})
	}
	out := &serverv1.ListFunctionsSummaryResponse{}
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
	// Load server-side UI overrides and overlay (server overrides take precedence)
	uiPath := os.Getenv("CROUPIER_UI_CONFIG")
	if uiPath == "" {
		// default lookup: single file or directory with multiple jsons
		// try common locations
		// 1) configs/ui/functions.override.json
		// 2) configs/ui/functions.json
		// 3) configs/ui (dir)
		// We merge in this order so that functions.override.json wins.
		// We'll feed all three paths to loader which merges in sequence.
		override1 := "configs/ui/functions.override.json"
		override2 := "configs/ui/functions.json"
		dir := "configs/ui"
		overrides := s.reg.LoadUIOverrides(override2, dir, override1)
		if len(overrides) > 0 {
			for fid, data := range overrides {
				if data == nil {
					continue
				}
				if _, ok := metaIdx[fid]; !ok {
					metaIdx[fid] = map[string]interface{}{}
				}
				for k, v := range data {
					metaIdx[fid][k] = v
				}
			}
		}
	} else {
		overrides := s.reg.LoadUIOverrides(uiPath)
		if len(overrides) > 0 {
			for fid, data := range overrides {
				if data == nil {
					continue
				}
				if _, ok := metaIdx[fid]; !ok {
					metaIdx[fid] = map[string]interface{}{}
				}
				for k, v := range data {
					metaIdx[fid][k] = v
				}
			}
		}
	}
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
		fd := &serverv1.FunctionDescriptor{
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

func parseI18n(v interface{}) *commonv1.I18NText {
	if v == nil {
		return nil
	}
	switch t := v.(type) {
	case map[string]interface{}:
		out := &commonv1.I18NText{}
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
		return &commonv1.I18NText{Zh: t}
	default:
		// Try JSON string
		if s, ok := v.(string); ok {
			var m map[string]string
			if err := json.Unmarshal([]byte(s), &m); err == nil {
				out := &commonv1.I18NText{En: m["en"], Zh: m["zh"]}
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

// validateManifest validates the manifest data against the loaded JSON Schema
func (s *Server) validateManifest(manifestData []byte) error {
	// Parse manifest data as JSON interface
	var manifest interface{}
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		st := status.New(codes.InvalidArgument, "manifest is not valid JSON")
		br := &errdetails.BadRequest{
			FieldViolations: []*errdetails.BadRequest_FieldViolation{{
				Field:       "manifest",
				Description: err.Error(),
			}},
		}
		st, _ = st.WithDetails(
			br,
			&errdetails.ErrorInfo{Reason: "PROVIDER_MANIFEST_INVALID_JSON", Domain: "croupier.server"},
		)
		return st.Err()
	}

	// Create document loader for the manifest
	documentLoader := gojsonschema.NewGoLoader(manifest)

	// Validate against schema
	result, err := s.schemaLoader.Validate(documentLoader)
	if err != nil {
		return status.Errorf(codes.Internal, "manifest schema validation error: %v", err)
	}

	// Check validation result
	if !result.Valid() {
		fieldViolations := make([]*errdetails.BadRequest_FieldViolation, 0, len(result.Errors()))
		for _, schemaErr := range result.Errors() {
			field := strings.TrimSpace(schemaErr.Field())
			field = strings.TrimPrefix(field, "(root)")
			field = strings.TrimPrefix(field, ".")
			if field == "" {
				field = "manifest"
			}
			desc := strings.TrimSpace(schemaErr.Description())
			if desc == "" {
				desc = strings.TrimSpace(schemaErr.String())
			}
			fieldViolations = append(fieldViolations, &errdetails.BadRequest_FieldViolation{
				Field:       field,
				Description: desc,
			})
		}

		st := status.New(codes.InvalidArgument, "manifest does not match provider manifest schema")
		br := &errdetails.BadRequest{FieldViolations: fieldViolations}
		st, _ = st.WithDetails(
			br,
			&errdetails.ErrorInfo{Reason: "PROVIDER_MANIFEST_SCHEMA_VIOLATION", Domain: "croupier.server"},
		)
		return st.Err()
	}

	return nil
}
