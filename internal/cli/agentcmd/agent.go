package agentcmd

import (
    "context"
    "crypto/tls"
    "crypto/x509"
    "encoding/json"
    "fmt"
    "io"
    "io/ioutil"
    "log"
    "net"
    "net/http"
    "archive/tar"
    "compress/gzip"
    "strings"
    "time"
    "os"
    "path/filepath"
    "mime/multipart"
    "bytes"
    "os/exec"
    "sync"

    "github.com/spf13/cobra"
    "github.com/spf13/viper"
    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials"
    "google.golang.org/grpc/keepalive"

    controlclient "github.com/cuihairu/croupier/internal/agent/control"
    controlv1 "github.com/cuihairu/croupier/gen/go/croupier/control/v1"
    functionv1 "github.com/cuihairu/croupier/gen/go/croupier/function/v1"
    agentfunc "github.com/cuihairu/croupier/internal/agent/function"
    localv1 "github.com/cuihairu/croupier/gen/go/croupier/agent/local/v1"
    locallib "github.com/cuihairu/croupier/internal/agent/local"
    localreg "github.com/cuihairu/croupier/internal/agent/registry"
    "github.com/cuihairu/croupier/internal/agent/jobs"
    tunn "github.com/cuihairu/croupier/internal/agent/tunnel"
    _ "github.com/cuihairu/croupier/internal/transport/jsoncodec"
    "github.com/cuihairu/croupier/internal/devcert"
    common "github.com/cuihairu/croupier/internal/cli/common"
)

func loadClientTLS(certFile, keyFile, caFile string, serverName string) (credentials.TransportCredentials, error) {
    cert, err := tls.LoadX509KeyPair(certFile, keyFile)
    if err != nil { return nil, err }
    caPEM, err := ioutil.ReadFile(caFile)
    if err != nil { return nil, err }
    pool := x509.NewCertPool()
    if !pool.AppendCertsFromPEM(caPEM) { return nil, err }
    cfg := &tls.Config{Certificates: []tls.Certificate{cert}, RootCAs: pool, ServerName: serverName}
    return credentials.NewTLS(cfg), nil
}

// New returns the `croupier agent` command.
func New() *cobra.Command {
    var cfgFile string
    var includes []string
    var profile string
    cmd := &cobra.Command{
        Use:   "agent",
        Short: "Run Croupier Agent",
        RunE: func(cmd *cobra.Command, args []string) error {
            v, err := common.LoadWithIncludes(cfgFile, includes)
            if err != nil { return err }
            v.SetEnvPrefix("CROUPIER_AGENT")
            v.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))
            v.AutomaticEnv()
            if v, err = common.ApplySectionAndProfile(v, "agent", profile); err != nil { return err }
            common.MergeLogSection(v)

            // logging setup
            common.SetupLoggerWithFile(
                v.GetString("log.level"),
                v.GetString("log.format"),
                v.GetString("log.file"),
                v.GetInt("log.max_size"),
                v.GetInt("log.max_backups"),
                v.GetInt("log.max_age"),
                v.GetBool("log.compress"),
            )

            localAddr := v.GetString("local_addr")
            serverAddr := v.GetString("server_addr")
            coreAddr := v.GetString("core_addr")
            serverName := v.GetString("server_name")
            cert := v.GetString("cert")
            key := v.GetString("key")
            ca := v.GetString("ca")
            insecureLocal := v.GetBool("insecure_local")
            agentID := v.GetString("agent_id")
            agentVersion := v.GetString("agent_version")
            gameID := v.GetString("game_id")
            env := v.GetString("env")
            httpAddr := v.GetString("http_addr")

            if serverAddr != "" {
                if coreAddr != "" && coreAddr != "127.0.0.1:8443" {
                    log.Printf("[warn] both --server_addr and --core_addr provided; using --server_addr=%s", serverAddr)
                }
                coreAddr = serverAddr
            } else if coreAddr != "" {
                log.Printf("[warn] --core_addr is deprecated; please use --server_addr")
            }

            // Validate config (non-strict) then auto-generate dev certs when not provided (DEV ONLY)
            if err := common.ValidateAgentConfig(v, false); err != nil { return err }
            // Auto-generate dev certs when not provided (DEV ONLY)
            if (cert == "" || key == "" || ca == "") && coreAddr != "" {
                out := "configs/dev"
                caCrt, caKey, err := devcert.EnsureDevCA(out)
                if err != nil { return err }
                agCrt, agKey, err := devcert.EnsureAgentCert(out, caCrt, caKey, agentID)
                if err != nil { return err }
                cert, key, ca = agCrt, agKey, caCrt
                log.Printf("[devcert] generated dev mTLS certs under %s (DEV ONLY)", out)
            }

            // Connect to Server with mTLS
            var dialOpt grpc.DialOption
            if cert != "" && key != "" && ca != "" {
                sni := serverName
                if sni == "" {
                    host := coreAddr
                    if i := strings.Index(host, "://"); i >= 0 { host = host[i+3:] }
                    if i := strings.LastIndex(host, ":"); i >= 0 { host = host[:i] }
                    sni = host
                }
                creds, err := loadClientTLS(cert, key, ca, sni)
                if err != nil { return err }
                dialOpt = grpc.WithTransportCredentials(creds)
            } else {
                return fmt.Errorf("missing TLS cert/key/ca; provide --cert/--key/--ca or set Insecure for dev")
            }

            coreConn, err := grpc.Dial(coreAddr, dialOpt, grpc.WithKeepaliveParams(keepalive.ClientParameters{Time: 30 * time.Second}), grpc.WithDefaultCallOptions(grpc.CallContentSubtype("json")))
            if err != nil { return err }
            defer coreConn.Close()

            // Bootstrap register/heartbeat
            go func() {
                cc := controlclient.NewClient(coreConn)
                fns := []*controlv1.FunctionDescriptor{}
                ctx := context.Background()
                cc.RegisterAndHeartbeat(ctx, agentID, agentVersion, localAddr, gameID, env, fns)
            }()

            // Local gRPC for game servers
            lis, err := net.Listen("tcp", localAddr)
            if err != nil { return err }

            var srv *grpc.Server
            if insecureLocal { srv = grpc.NewServer() } else { return fmt.Errorf("secure local server not implemented; set --insecure_local") }

            lstore := localreg.NewLocalStore()
            exec := jobs.NewExecutor()
            functionv1.RegisterFunctionServiceServer(srv, agentfunc.NewServer(lstore, exec))
            lserver := locallib.NewServer(lstore, controlv1.NewControlServiceClient(coreConn), agentID, agentVersion, localAddr, gameID, env, exec)
            localv1.RegisterLocalControlServiceServer(srv, lserver)

            // optional assignments polling for downlink preview and lightweight adapter control
            var promSup *adapterSupervisor
            var httpSup *adapterSupervisor
            if api := v.GetString("assignments_api"); api != "" && gameID != "" {
                go func(){
                    interval := time.Duration(v.GetInt("assignments_poll_sec")) * time.Second
                    if interval <= 0 { interval = 30 * time.Second }
                    downDir := v.GetString("downlink_dir")
                    promCmd := splitCmd(v.GetString("adapter_prom_cmd"))
                    httpCmd := splitCmd(v.GetString("adapter_http_cmd"))
                    // build base env for adapters
                    baseEnv := buildAdapterEnv(os.Environ(), map[string]string{
                        "CROUPIER_AGENT_ID": agentID,
                        "CROUPIER_GAME_ID":  gameID,
                        "CROUPIER_ENV":      env,
                        // Best-effort passthrough common variables (if present)
                        "PROM_URL":          os.Getenv("PROM_URL"),
                        "ASSIGNMENTS_API":   api,
                    })
                    promSup = newAdapterSupervisor("prom", promCmd, baseEnv)
                    httpSup = newAdapterSupervisor("http", httpCmd, baseEnv)
                    for {
                        func(){
                            req, _ := http.NewRequest("GET", api+"/api/assignments?game_id="+gameID+"&env="+env, nil)
                            resp, err := http.DefaultClient.Do(req)
                            if err != nil { log.Printf("assignments poll error: %v", err); return }
                            defer resp.Body.Close()
                            if resp.StatusCode/100 != 2 { b,_ := io.ReadAll(resp.Body); log.Printf("assignments poll failed: %s", string(b)); return }
                            var out struct{ Assignments map[string][]string `json:"assignments"` }
                            if err := json.NewDecoder(resp.Body).Decode(&out); err != nil { log.Printf("assignments decode: %v", err); return }
                            var fns []string
                            for _, arr := range out.Assignments { fns = append(fns, arr...) }
                            lserver.UpdateAllowed(fns)
                            log.Printf("assignments updated: %d functions", len(fns))
                            // optional adapter start/stop (supervised, best-effort)
                            if len(fns) == 0 { // empty -> allow all
                                promSup.SetDesired(true)
                                httpSup.SetDesired(true)
                            } else {
                                wantProm, wantHttp := calcAdapterNeeds(fns)
                                promSup.SetDesired(len(promCmd) > 0 && wantProm)
                                httpSup.SetDesired(len(httpCmd) > 0 && wantHttp)
                            }
                            if downDir != "" {
                                // fetch current pack export and write to downDir/pack.tgz (and extract)
                                if err := os.MkdirAll(downDir, 0o755); err != nil { log.Printf("downlink dir: %v", err) } else {
                                    packPath := filepath.Join(downDir, "pack.tgz")
                                    if err := downloadAndExtractPack(api+"/api/packs/export", packPath, downDir); err != nil {
                                        log.Printf("downlink export failed: %v", err)
                                    } else {
                                        log.Printf("downlink export saved to %s", downDir)
                                        // optionally notify server to reload (best-effort) or import the pack we just downloaded
                                        if err := uploadPack(api+"/api/packs/import", packPath); err != nil {
                                            // fallback to reload
                                            _, _ = http.Post(api+"/api/packs/reload", "application/json", nil)
                                        }
                                        // verify server responds to packs/list (basic reload check)
                                        if err := verifyServerPacksList(api, 5*time.Second); err != nil {
                                            log.Printf("downlink verify packs/list failed: %v", err)
                                        } else { log.Printf("downlink verify packs/list ok") }
                                    }
                                }
                            }
                        }()
                        time.Sleep(interval)
                    }
                }()
            }

            // Tunnel to Server/Edge
            go func(){
                t := tunn.NewClient(coreAddr, agentID, gameID, env, localAddr)
                backoff := time.Second
                for {
                    if err := t.Start(context.Background()); err != nil { log.Printf("tunnel disconnected: %v", err) }
                    time.Sleep(backoff)
                    if backoff < 30*time.Second { backoff *= 2 }
                    tunn.IncReconnect()
                }
            }()

            // HTTP health/metrics
            go func(){
                mux := http.NewServeMux()
                mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request){ w.WriteHeader(http.StatusOK); _,_ = w.Write([]byte("ok")) })
                mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request){
                    mp := lstore.List(); total := 0; fns := 0
                    for _, arr := range mp { fns++; total += len(arr) }
                    adapters := map[string]any{}
                    if promSup != nil { adapters["prom"] = promSup.Stats() }
                    if httpSup != nil { adapters["http"] = httpSup.Stats() }
                    _ = json.NewEncoder(w).Encode(map[string]any{
                        "functions": fns,
                        "instances": total,
                        "tunnel_reconnects": tunn.Reconnects(),
                        "logs": common.GetLogCounters(),
                        "adapters": adapters,
                    })
                })
                mux.HandleFunc("/metrics.prom", func(w http.ResponseWriter, r *http.Request){
                    w.Header().Set("Content-Type", "text/plain; version=0.0.4")
                    mp := lstore.List(); total := 0
                    for _, arr := range mp { total += len(arr) }
                    fmt.Fprintf(w, "# TYPE croupier_agent_instances gauge\n")
                    fmt.Fprintf(w, "croupier_agent_instances %d\n", total)
                    fmt.Fprintf(w, "# TYPE croupier_tunnel_reconnects counter\n")
                    fmt.Fprintf(w, "croupier_tunnel_reconnects %d\n", tunn.Reconnects())
                    lc := common.GetLogCounters()
                    fmt.Fprintf(w, "# TYPE croupier_logs_total counter\n")
                    fmt.Fprintf(w, "croupier_logs_total{level=\"debug\"} %d\n", lc["debug"])
                    fmt.Fprintf(w, "croupier_logs_total{level=\"info\"} %d\n", lc["info"])
                    fmt.Fprintf(w, "croupier_logs_total{level=\"warn\"} %d\n", lc["warn"])
                    fmt.Fprintf(w, "croupier_logs_total{level=\"error\"} %d\n", lc["error"])
                    // adapter metrics
                    if promSup != nil {
                        st := promSup.Stats()
                        fmt.Fprintf(w, "# TYPE croupier_adapter_running gauge\n")
                        fmt.Fprintf(w, "croupier_adapter_running{adapter=\"prom\"} %d\n", b2i(st.Running))
                        fmt.Fprintf(w, "# TYPE croupier_adapter_restarts_total counter\n")
                        fmt.Fprintf(w, "croupier_adapter_restarts_total{adapter=\"prom\"} %d\n", st.Restarts)
                    }
                    if httpSup != nil {
                        st := httpSup.Stats()
                        fmt.Fprintf(w, "croupier_adapter_running{adapter=\"http\"} %d\n", b2i(st.Running))
                        fmt.Fprintf(w, "croupier_adapter_restarts_total{adapter=\"http\"} %d\n", st.Restarts)
                    }
                })
                log.Printf("agent http listening on %s", httpAddr)
                _ = http.ListenAndServe(httpAddr, mux)
            }()
            log.Printf("croupier-agent listening on %s; connected to server %s", localAddr, coreAddr)
            if err := srv.Serve(lis); err != nil { return err }
            return nil
        },
    }
    cmd.Flags().StringVar(&cfgFile, "config", "", "config file (yaml), supports top-level 'agent:' section")
    cmd.Flags().StringSliceVar(&includes, "config-include", nil, "additional config files to merge in order (overrides base)")
    cmd.Flags().StringVar(&profile, "profile", "", "profile name under 'profiles:' to overlay")
    cmd.Flags().String("local_addr", ":19090", "local gRPC listen for game servers")
    cmd.Flags().String("server_addr", "", "server grpc address (alias for --core_addr)")
    cmd.Flags().String("core_addr", "127.0.0.1:8443", "server grpc address (deprecated)")
    cmd.Flags().String("server_name", "", "tls server name (SNI)")
    cmd.Flags().String("cert", "", "client mTLS cert file")
    cmd.Flags().String("key", "", "client mTLS key file")
    cmd.Flags().String("ca", "", "ca cert file to verify server")
    cmd.Flags().Bool("insecure_local", true, "use insecure for local listener (development)")
    cmd.Flags().String("agent_id", "agent-1", "agent id")
    cmd.Flags().String("agent_version", "0.1.0", "agent version")
    cmd.Flags().String("game_id", "", "game id (required if server enforces whitelist)")
    cmd.Flags().String("env", "", "environment (optional) e.g. prod/stage/test")
    cmd.Flags().String("http_addr", ":19091", "agent http listen for health/metrics")
    cmd.Flags().String("assignments_api", "", "server http base for assignments polling (e.g., http://localhost:8080)")
    cmd.Flags().Int("assignments_poll_sec", 30, "assignments polling interval seconds")
    cmd.Flags().String("downlink_dir", "", "directory to save/export current pack when assignments update (optional)")
    cmd.Flags().String("adapter_prom_cmd", "", "command to start prom adapter (e.g., './bin/prom-adapter' or 'go run ./adapters/prom')")
    cmd.Flags().String("adapter_http_cmd", "", "command to start http adapter (e.g., './bin/http-adapter' or 'go run ./adapters/http')")
    cmd.Flags().String("log.level", "info", "log level: debug|info|warn|error")
    cmd.Flags().String("log.format", "console", "log format: console|json")
    cmd.Flags().String("log.file", "", "log file path (if set, enable rotation)")
    cmd.Flags().Int("log.max_size", 100, "max size of log file in MB before rotation")
    cmd.Flags().Int("log.max_backups", 7, "max number of old log files to retain")
    cmd.Flags().Int("log.max_age", 7, "max age (days) to retain old log files")
    cmd.Flags().Bool("log.compress", true, "compress rotated log files")
    _ = viper.BindPFlags(cmd.Flags())
    return cmd
}

// downloadAndExtractPack downloads a tar.gz from url to dstFile and extracts selected entries into dir.
func downloadAndExtractPack(url, dstFile, dir string) error {
    resp, err := http.Get(url)
    if err != nil { return err }
    defer resp.Body.Close()
    if resp.StatusCode/100 != 2 { b,_ := io.ReadAll(resp.Body); return fmt.Errorf("download failed: %s", string(b)) }
    // save
    f, err := os.Create(dstFile)
    if err != nil { return err }
    if _, err := io.Copy(f, resp.Body); err != nil { f.Close(); return err }
    f.Close()
    // extract
    rf, err := os.Open(dstFile)
    if err != nil { return err }
    defer rf.Close()
    gz, err := gzip.NewReader(rf)
    if err != nil { return err }
    defer gz.Close()
    tr := tar.NewReader(gz)
    for {
        hdr, err := tr.Next()
        if err == io.EOF { break }
        if err != nil { return err }
        name := hdr.Name
        // Only extract descriptors/ui/manifest.json/web-plugin/*.js and *.pb at root
        if !(strings.HasPrefix(name, "descriptors/") || strings.HasPrefix(name, "ui/") || strings.HasPrefix(name, "web-plugin/") || name == "manifest.json" || strings.HasSuffix(name, ".pb")) {
            continue
        }
        outPath := filepath.Join(dir, filepath.FromSlash(name))
        if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil { return err }
        out, err := os.Create(outPath)
        if err != nil { return err }
        if _, err := io.Copy(out, tr); err != nil { out.Close(); return err }
        out.Close()
    }
    return nil
}

// adapter supervisor: manages adapter process with graceful stop and backoff restarts (dev-grade)
type adapterSupervisor struct {
    name     string
    args     []string
    env      []string
    mu       sync.Mutex
    cmd      *exec.Cmd
    desired  bool
    running  bool
    backoff  time.Duration
    stopping bool
    restarts int64
    lastStart time.Time
}

func newAdapterSupervisor(name string, args []string, env []string) *adapterSupervisor {
    return &adapterSupervisor{name: name, args: args, env: env, backoff: time.Second}
}

func (s *adapterSupervisor) SetDesired(want bool) {
    s.mu.Lock()
    s.desired = want
    // if we want running and not running, try start; if not want but running, stop gracefully
    if want {
        if !s.running { go s.startLoop() }
    } else {
        if s.running { go s.stopGraceful(3 * time.Second) }
    }
    s.mu.Unlock()
}

func (s *adapterSupervisor) startLoop() {
    s.mu.Lock()
    if s.running || !s.desired || len(s.args) == 0 { s.mu.Unlock(); return }
    // spawn process
    cmd := exec.Command(s.args[0], s.args[1:]...)
    cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr
    cmd.Env = s.env
    if err := cmd.Start(); err != nil {
        log.Printf("adapter %s start error: %v", s.name, err)
        s.mu.Unlock()
        time.Sleep(s.backoff)
        if s.backoff < 30*time.Second { s.backoff *= 2 }
        // retry if still desired
        s.mu.Lock(); defer s.mu.Unlock()
        if s.desired { go s.startLoop() }
        return
    }
    s.cmd = cmd
    s.running = true
    s.backoff = time.Second // reset backoff on successful start
    s.lastStart = time.Now()
    log.Printf("adapter %s started: %s", s.name, strings.Join(s.args, " "))
    s.mu.Unlock()

    // wait and handle exit
    err := cmd.Wait()
    s.mu.Lock()
    s.running = false
    s.cmd = nil
    s.mu.Unlock()
    if s.stopping {
        log.Printf("adapter %s exited (stopped)", s.name)
        s.stopping = false
        return
    }
    if err != nil { log.Printf("adapter %s exited: %v", s.name, err) } else { log.Printf("adapter %s exited", s.name) }
    // count restarts only when unexpected exit
    s.mu.Lock(); s.restarts++; s.mu.Unlock()
    // backoff and restart if still desired
    time.Sleep(s.backoff)
    if s.backoff < 30*time.Second { s.backoff *= 2 }
    s.mu.Lock(); want := s.desired; s.mu.Unlock()
    if want { s.startLoop() }
}

func (s *adapterSupervisor) stopGraceful(timeout time.Duration) {
    s.mu.Lock()
    if s.cmd == nil || !s.running { s.mu.Unlock(); return }
    s.stopping = true
    cmd := s.cmd
    s.mu.Unlock()
    // try SIGTERM first
    _ = cmd.Process.Signal(os.Interrupt)
    done := make(chan struct{}, 1)
    go func(){ _ = cmd.Wait(); done <- struct{}{} }()
    select {
    case <-done:
    case <-time.After(timeout):
        _ = cmd.Process.Kill()
    }
}

func buildAdapterEnv(base []string, extra map[string]string) []string {
    // copy base first
    out := append([]string{}, base...)
    for k, v := range extra {
        if v == "" { continue }
        out = append(out, fmt.Sprintf("%s=%s", k, v))
    }
    return out
}

// AdapterStats is a snapshot of adapter supervisor state for metrics.
type AdapterStats struct{
    Name string `json:"name"`
    Running bool `json:"running"`
    Desired bool `json:"desired"`
    Restarts int64 `json:"restarts"`
    BackoffSec int `json:"backoff_sec"`
    UptimeSec int `json:"uptime_sec"`
}

func (s *adapterSupervisor) Stats() AdapterStats {
    s.mu.Lock(); defer s.mu.Unlock()
    up := 0
    if s.running && !s.lastStart.IsZero() { up = int(time.Since(s.lastStart).Seconds()) }
    return AdapterStats{
        Name: s.name,
        Running: s.running,
        Desired: s.desired,
        Restarts: s.restarts,
        BackoffSec: int(s.backoff.Seconds()),
        UptimeSec: up,
    }
}

func b2i(b bool) int { if b { return 1 } ; return 0 }
func splitCmd(s string) []string {
    if s == "" { return nil }
    return strings.Fields(s)
}
func calcAdapterNeeds(fns []string) (wantProm, wantHttp bool) {
    for _, id := range fns {
        if strings.HasPrefix(id, "prom.") { wantProm = true }
        if strings.HasPrefix(id, "http.") || strings.HasPrefix(id, "grafana.") || strings.HasPrefix(id, "alertmanager.") { wantHttp = true }
    }
    return
}

// uploadPack posts a pack tar.gz to server import endpoint.
func uploadPack(importURL, path string) error {
    f, err := os.Open(path)
    if err != nil { return err }
    defer f.Close()
    var body bytes.Buffer
    mw := multipart.NewWriter(&body)
    fw, err := mw.CreateFormFile("file", filepath.Base(path))
    if err != nil { return err }
    if _, err := io.Copy(fw, f); err != nil { return err }
    mw.Close()
    req, _ := http.NewRequest("POST", importURL, &body)
    req.Header.Set("Content-Type", mw.FormDataContentType())
    resp, err := http.DefaultClient.Do(req)
    if err != nil { return err }
    defer resp.Body.Close()
    if resp.StatusCode/100 != 2 { b,_ := io.ReadAll(resp.Body); return fmt.Errorf("import failed: %s", string(b)) }
    return nil
}

// verifyServerPacksList polls /api/packs/list to ensure server responds within timeout.
func verifyServerPacksList(api string, timeout time.Duration) error {
    deadline := time.Now().Add(timeout)
    for {
        resp, err := http.Get(api + "/api/packs/list")
        if err == nil {
            io.Copy(io.Discard, resp.Body)
            resp.Body.Close()
            if resp.StatusCode/100 == 2 { return nil }
        }
        if time.Now().After(deadline) { break }
        time.Sleep(200 * time.Millisecond)
    }
    return fmt.Errorf("packs/list not confirmed within %s", timeout)
}
