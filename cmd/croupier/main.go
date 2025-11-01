package main

import (
    "bytes"
    "encoding/json"
    "fmt"
    "log"
    "os"

    "github.com/spf13/cobra"
    "github.com/spf13/viper"
    servercmd "github.com/cuihairu/croupier/internal/cli/servercmd"
    agentcmd "github.com/cuihairu/croupier/internal/cli/agentcmd"
    edgecmd "github.com/cuihairu/croupier/internal/cli/edgecmd"
    common "github.com/cuihairu/croupier/internal/cli/common"
    "net/http"
    "mime/multipart"
    "path/filepath"
    "io"
    "compress/gzip"
    "archive/tar"
    "net/url"
    "strings"
    "os/exec"
)

// sanitize turns an id into a file-name friendly slug (keep [a-zA-Z0-9._-])
func sanitize(id string) string {
    out := make([]rune, 0, len(id))
    for _, r := range id {
        switch {
        case r >= 'a' && r <= 'z':
            out = append(out, r)
        case r >= 'A' && r <= 'Z':
            out = append(out, r)
        case r >= '0' && r <= '9':
            out = append(out, r)
        case r == '.' || r == '-' || r == '_':
            out = append(out, r)
        default:
            out = append(out, '-')
        }
    }
    return string(out)
}

func main() {
    root := &cobra.Command{Use: "croupier", Short: "Croupier unified CLI"}

    // Subcommands
    root.AddCommand(servercmd.New())
    root.AddCommand(agentcmd.New())
    root.AddCommand(edgecmd.New())

    // completion
    comp := &cobra.Command{Use: "completion [bash|zsh|fish|powershell]", Short: "Generate shell completion"}
    comp.Run = func(cmd *cobra.Command, args []string) {
        if len(args) == 0 { log.Fatalf("specify a shell: bash|zsh|fish|powershell") }
        sh := args[0]
        switch sh {
        case "bash": root.GenBashCompletion(os.Stdout)
        case "zsh": root.GenZshCompletion(os.Stdout)
        case "fish": root.GenFishCompletion(os.Stdout, true)
        case "powershell": root.GenPowerShellCompletionWithDesc(os.Stdout)
        default: log.Fatalf("unknown shell: %s", sh)
        }
    }
    root.AddCommand(comp)

    // config test (minimal validation)
    cfgTest := &cobra.Command{Use: "config test", Short: "Validate and print effective config"}
    var cfgFile, section string
    var includes []string
    var profile string
    cfgTest.Flags().StringVar(&cfgFile, "config", "", "config file path")
    cfgTest.Flags().StringVar(&section, "section", "", "optional section: server|agent")
    cfgTest.Flags().StringSliceVar(&includes, "config-include", nil, "additional config files to merge in order")
    cfgTest.Flags().StringVar(&profile, "profile", "", "profile name under 'profiles:' to overlay")
    cfgTest.RunE = func(cmd *cobra.Command, args []string) error {
        if cfgFile == "" { return fmt.Errorf("--config required") }
        v, err := common.LoadWithIncludes(cfgFile, includes)
        if err != nil { return err }
        // Prepare snapshot helper
        snapshot := func(base *viper.Viper, sect string) error {
            if base == nil { return fmt.Errorf("section %s not found", sect) }
            // merge log subsection for snapshot
            vv, err := common.ApplySectionAndProfile(base, sect, profile)
            if err != nil { return err }
            common.MergeLogSection(vv)
            m := vv.AllSettings()
            // validate strictly
            var verr error
            switch sect {
            case "server": verr = common.ValidateServerConfig(vv, true)
            case "agent": verr = common.ValidateAgentConfig(vv, true)
            }
            if verr != nil { return verr }
            // print pretty JSON
            enc := json.NewEncoder(os.Stdout)
            enc.SetIndent("", "  ")
            return enc.Encode(map[string]any{"section": sect, "effective": m})
        }
        switch section {
        case "server": return snapshot(v, "server")
        case "agent": return snapshot(v, "agent")
        case "":
            if err := snapshot(v, "server"); err == nil { return nil }
            if err := snapshot(v, "agent"); err == nil { return nil }
            return fmt.Errorf("no valid section found; specify --section")
        default:
            return fmt.Errorf("unknown section: %s", section)
        }
    }
    root.AddCommand(cfgTest)

    // packs import (POST to server /api/packs/import)
    packs := &cobra.Command{Use: "packs"}
    importCmd := &cobra.Command{Use: "import <pack.tgz>", Short: "Import a function pack (fds+descriptors) into the running server"}
    var api string
    importCmd.Flags().StringVar(&api, "api", "http://localhost:8080", "Server HTTP address")
    importCmd.RunE = func(cmd *cobra.Command, args []string) error {
        if len(args) == 0 { return fmt.Errorf("pack path required") }
        path := args[0]
        // if a directory is provided, tar.gz it into memory first
        var reader io.Reader
        var filename string
        if fi, err := os.Stat(path); err == nil && fi.IsDir() {
            // tar.gz the directory
            var buf bytes.Buffer
            gz := gzip.NewWriter(&buf)
            tw := tar.NewWriter(gz)
            err := filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
                if err != nil { return err }
                if info.IsDir() { return nil }
                rel, _ := filepath.Rel(path, p)
                b, err := os.ReadFile(p)
                if err != nil { return err }
                hdr := &tar.Header{Name: filepath.ToSlash(rel), Mode: 0644, Size: int64(len(b))}
                if err := tw.WriteHeader(hdr); err != nil { return err }
                if _, err := tw.Write(b); err != nil { return err }
                return nil
            })
            if err != nil { return err }
            if err := tw.Close(); err != nil { return err }
            if err := gz.Close(); err != nil { return err }
            reader = bytes.NewReader(buf.Bytes())
            filename = filepath.Base(path) + ".pack.tgz"
        } else {
            f, err := os.Open(path)
            if err != nil { return err }
            defer f.Close()
            reader = f
            filename = filepath.Base(path)
        }
        body := &bytes.Buffer{}
        mw := multipart.NewWriter(body)
        fw, err := mw.CreateFormFile("file", filename)
        if err != nil { return err }
        if _, err := io.Copy(fw, reader); err != nil { return err }
        mw.Close()
        req, _ := http.NewRequest("POST", api+"/api/packs/import", body)
        req.Header.Set("Content-Type", mw.FormDataContentType())
        resp, err := http.DefaultClient.Do(req)
        if err != nil { return err }
        defer resp.Body.Close()
        if resp.StatusCode/100 != 2 { b,_ := io.ReadAll(resp.Body); return fmt.Errorf("import failed: %s", string(b)) }
        fmt.Println("import ok")
        return nil
    }
    packs.AddCommand(importCmd)
    listCmd := &cobra.Command{Use: "list", Short: "List current packs (manifest) from the server"}
    listCmd.RunE = func(cmd *cobra.Command, args []string) error {
        req, _ := http.NewRequest("GET", api+"/api/packs/list", nil)
        resp, err := http.DefaultClient.Do(req)
        if err != nil { return err }
        defer resp.Body.Close()
        if resp.StatusCode/100 != 2 { b,_ := io.ReadAll(resp.Body); return fmt.Errorf("list failed: %s", string(b)) }
        io.Copy(os.Stdout, resp.Body)
        return nil
    }
    packs.AddCommand(listCmd)
    exportCmd := &cobra.Command{Use: "export <output.tgz>", Short: "Export current pack from the server as tar.gz"}
    exportCmd.Args = cobra.ExactArgs(1)
    var showETag bool
    exportCmd.Flags().BoolVar(&showETag, "show-etag", false, "Print ETag header of the exported pack")
    exportCmd.RunE = func(cmd *cobra.Command, args []string) error {
        outPath := args[0]
        req, _ := http.NewRequest("GET", api+"/api/packs/export", nil)
        resp, err := http.DefaultClient.Do(req)
        if err != nil { return err }
        defer resp.Body.Close()
        if resp.StatusCode/100 != 2 { b,_ := io.ReadAll(resp.Body); return fmt.Errorf("export failed: %s", string(b)) }
        f, err := os.Create(outPath)
        if err != nil { return err }
        defer f.Close()
        if _, err := io.Copy(f, resp.Body); err != nil { return err }
        if showETag {
            fmt.Println("ETag:", resp.Header.Get("ETag"))
        }
        fmt.Println("exported:", outPath)
        return nil
    }
    packs.AddCommand(exportCmd)
    reloadCmd := &cobra.Command{Use: "reload", Short: "Reload pack descriptors and fds from server pack dir"}
    reloadCmd.RunE = func(cmd *cobra.Command, args []string) error {
        req, _ := http.NewRequest("POST", api+"/api/packs/reload", nil)
        resp, err := http.DefaultClient.Do(req)
        if err != nil { return err }
        defer resp.Body.Close()
        if resp.StatusCode/100 != 2 { b,_ := io.ReadAll(resp.Body); return fmt.Errorf("reload failed: %s", string(b)) }
        fmt.Println("ok")
        return nil
    }
    packs.AddCommand(reloadCmd)
    inspectCmd := &cobra.Command{Use: "inspect <pack.tgz>", Short: "Inspect pack contents (manifest/descriptors/ui)"}
    inspectCmd.RunE = func(cmd *cobra.Command, args []string) error {
        if len(args) == 0 { return fmt.Errorf("pack path required") }
        path := args[0]
        f, err := os.Open(path)
        if err != nil { return err }
        defer f.Close()
        gz, err := gzip.NewReader(f)
        if err != nil { return err }
        defer gz.Close()
        tr := tar.NewReader(gz)
        type entry struct{ Name string; Size int64 }
        entries := []entry{}
        var manifest []byte
        for {
            hdr, err := tr.Next()
            if err == io.EOF { break }
            if err != nil { return err }
            if hdr.Name == "manifest.json" {
                b, _ := io.ReadAll(tr)
                manifest = b
            } else {
                entries = append(entries, entry{Name: hdr.Name, Size: hdr.Size})
            }
        }
        if len(manifest) > 0 {
            var m any
            _ = json.Unmarshal(manifest, &m)
            enc := json.NewEncoder(os.Stdout)
            enc.SetIndent("", "  ")
            fmt.Println("manifest.json:")
            _ = enc.Encode(m)
        }
        fmt.Println("files:")
        for _, e := range entries {
            fmt.Printf(" - %s (%d bytes)\n", e.Name, e.Size)
        }
        return nil
    }
    packs.AddCommand(inspectCmd)
    genCmd := &cobra.Command{Use: "gen", Short: "Generate pack via protoc-gen-croupier (requires protoc)"}
    genCmd.RunE = func(cmd *cobra.Command, args []string) error {
        // Try running the helper script
        script := "scripts/generate-pack.sh"
        if _, err := os.Stat(script); err != nil {
            return fmt.Errorf("helper script not found: %s", script)
        }
        c := execCommand(script)
        c.Stdout = os.Stdout
        c.Stderr = os.Stderr
        return c.Run()
    }
    packs.AddCommand(genCmd)
    validateCmd := &cobra.Command{Use: "validate <pack.tgz|dir>", Short: "Validate pack structure and basic consistency"}
    validateCmd.RunE = func(cmd *cobra.Command, args []string) error {
        if len(args) == 0 { return fmt.Errorf("path required") }
        path := args[0]
        type fsEntry struct{ Name string; Data []byte }
        files := map[string][]byte{}
        // load from tar.gz or dir
        if fi, err := os.Stat(path); err == nil && fi.IsDir() {
            // directory mode
            err := filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
                if err != nil { return err }
                if info.IsDir() { return nil }
                b, err := os.ReadFile(p)
                if err != nil { return err }
                rel, _ := filepath.Rel(path, p)
                files[filepath.ToSlash(rel)] = b
                return nil
            })
            if err != nil { return err }
        } else {
            f, err := os.Open(path)
            if err != nil { return err }
            defer f.Close()
            gz, err := gzip.NewReader(f)
            if err != nil { return err }
            defer gz.Close()
            tr := tar.NewReader(gz)
            for {
                hdr, err := tr.Next()
                if err == io.EOF { break }
                if err != nil { return err }
                b, _ := io.ReadAll(tr)
                name := strings.TrimPrefix(hdr.Name, "./")
                files[name] = b
            }
        }
        // basic checks
        if _, ok := files["manifest.json"]; !ok { return fmt.Errorf("missing manifest.json") }
        if _, ok := files["fds.pb"]; !ok { return fmt.Errorf("missing fds.pb") }
        var manifest struct{ Functions []struct{ ID string `json:"id"` } `json:"functions"` }
        if err := json.Unmarshal(files["manifest.json"], &manifest); err != nil { return fmt.Errorf("bad manifest.json: %w", err) }
        missing := []string{}
        for _, f := range manifest.Functions {
            name := "descriptors/" + sanitize(f.ID) + ".json"
            if _, ok := files[name]; !ok {
                missing = append(missing, name)
            }
        }
        if len(missing) > 0 {
            fmt.Println("missing descriptors:")
            for _, m := range missing { fmt.Println(" -", m) }
            return fmt.Errorf("pack invalid")
        }
        fmt.Println("pack valid")
        return nil
    }
    packs.AddCommand(validateCmd)
    root.AddCommand(packs)

    // registry CLI (introspection)
    registry := &cobra.Command{Use: "registry", Short: "Registry introspection"}
    var apiReg string
    registry.PersistentFlags().StringVar(&apiReg, "api", "http://localhost:8080", "Server HTTP address")
    summary := &cobra.Command{Use: "summary", Short: "Show agents and functions"}
    summary.RunE = func(cmd *cobra.Command, args []string) error {
        req, _ := http.NewRequest("GET", apiReg+"/api/registry", nil)
        resp, err := http.DefaultClient.Do(req)
        if err != nil { return err }
        defer resp.Body.Close()
        if resp.StatusCode/100 != 2 { b,_ := io.ReadAll(resp.Body); return fmt.Errorf("get registry failed: %s", string(b)) }
        io.Copy(os.Stdout, resp.Body)
        return nil
    }
    registry.AddCommand(summary)
    instances := &cobra.Command{Use: "instances", Short: "List function instances across agents"}
    var fidArg, gidArg string
    instances.Flags().StringVar(&fidArg, "function_id", "", "function id")
    instances.Flags().StringVar(&gidArg, "game_id", "", "game id")
    instances.RunE = func(cmd *cobra.Command, args []string) error {
        qs := make(url.Values)
        if fidArg != "" { qs.Set("function_id", fidArg) }
        if gidArg != "" { qs.Set("game_id", gidArg) }
        req, _ := http.NewRequest("GET", apiReg+"/api/function_instances?"+qs.Encode(), nil)
        resp, err := http.DefaultClient.Do(req)
        if err != nil { return err }
        defer resp.Body.Close()
        if resp.StatusCode/100 != 2 { b,_ := io.ReadAll(resp.Body); return fmt.Errorf("get instances failed: %s", string(b)) }
        io.Copy(os.Stdout, resp.Body)
        return nil
    }
    registry.AddCommand(instances)
    root.AddCommand(registry)

    // games CLI
    gamesCmd := &cobra.Command{Use: "games", Short: "Manage allowed games (server whitelist)"}
    var gamesAPI, gamesToken string
    gamesCmd.PersistentFlags().StringVar(&gamesAPI, "api", "http://localhost:8080", "Server HTTP address")
    gamesCmd.PersistentFlags().StringVar(&gamesToken, "token", os.Getenv("CROUPIER_TOKEN"), "JWT token (or set CROUPIER_TOKEN env)")
    gamesList := &cobra.Command{Use: "list", Short: "List allowed games"}
    gamesList.RunE = func(cmd *cobra.Command, args []string) error {
        req, _ := http.NewRequest("GET", gamesAPI+"/api/games", nil)
        if gamesToken != "" { req.Header.Set("Authorization", "Bearer "+gamesToken) }
        resp, err := http.DefaultClient.Do(req)
        if err != nil { return err }
        defer resp.Body.Close()
        if resp.StatusCode/100 != 2 { b,_ := io.ReadAll(resp.Body); return fmt.Errorf("list games failed: %s", string(b)) }
        io.Copy(os.Stdout, resp.Body)
        return nil
    }
    gamesCmd.AddCommand(gamesList)
    gamesAdd := &cobra.Command{Use: "add", Short: "Add allowed game (requires games:manage)"}
    var gidFlag, envFlag string
    gamesAdd.Flags().StringVar(&gidFlag, "game_id", "", "game id")
    gamesAdd.Flags().StringVar(&envFlag, "env", "", "env")
    gamesAdd.RunE = func(cmd *cobra.Command, args []string) error {
        if gidFlag == "" { return fmt.Errorf("--game_id required") }
        b,_ := json.Marshal(map[string]string{"game_id": gidFlag, "env": envFlag})
        req, _ := http.NewRequest("POST", gamesAPI+"/api/games", bytes.NewReader(b))
        req.Header.Set("Content-Type", "application/json")
        if gamesToken != "" { req.Header.Set("Authorization", "Bearer "+gamesToken) }
        resp, err := http.DefaultClient.Do(req)
        if err != nil { return err }
        defer resp.Body.Close()
        if resp.StatusCode/100 != 2 { c,_ := io.ReadAll(resp.Body); return fmt.Errorf("add game failed: %s", string(c)) }
        fmt.Println("ok")
        return nil
    }
    gamesCmd.AddCommand(gamesAdd)
    root.AddCommand(gamesCmd)

    // assignments CLI
    assigns := &cobra.Command{Use: "assignments", Short: "Manage function assignments per game/env"}
    var assignsAPI string
    assigns.PersistentFlags().StringVar(&assignsAPI, "api", "http://localhost:8080", "Server HTTP address")
    assignsList := &cobra.Command{Use: "list", Short: "List assignments"}
    var alGid, alEnv string
    assignsList.Flags().StringVar(&alGid, "game_id", "", "filter by game id")
    assignsList.Flags().StringVar(&alEnv, "env", "", "filter by env")
    assignsList.RunE = func(cmd *cobra.Command, args []string) error {
        qs := make(url.Values)
        if alGid != "" { qs.Set("game_id", alGid) }
        if alEnv != "" { qs.Set("env", alEnv) }
        req, _ := http.NewRequest("GET", assignsAPI+"/api/assignments?"+qs.Encode(), nil)
        resp, err := http.DefaultClient.Do(req)
        if err != nil { return err }
        defer resp.Body.Close()
        if resp.StatusCode/100 != 2 { b,_ := io.ReadAll(resp.Body); return fmt.Errorf("list failed: %s", string(b)) }
        io.Copy(os.Stdout, resp.Body)
        return nil
    }
    assigns.AddCommand(assignsList)
    assignsSet := &cobra.Command{Use: "set", Short: "Set assignments for game/env"}
    var asGid, asEnv string
    var asFns []string
    assignsSet.Flags().StringVar(&asGid, "game_id", "", "game id")
    assignsSet.Flags().StringVar(&asEnv, "env", "", "env")
    assignsSet.Flags().StringSliceVar(&asFns, "functions", nil, "function ids")
    assignsSet.RunE = func(cmd *cobra.Command, args []string) error {
        if asGid == "" { return fmt.Errorf("--game_id required") }
        b,_ := json.Marshal(map[string]any{"game_id": asGid, "env": asEnv, "functions": asFns})
        req, _ := http.NewRequest("POST", assignsAPI+"/api/assignments", bytes.NewReader(b))
        req.Header.Set("Content-Type", "application/json")
        resp, err := http.DefaultClient.Do(req)
        if err != nil { return err }
        defer resp.Body.Close()
        if resp.StatusCode/100 != 2 { c,_ := io.ReadAll(resp.Body); return fmt.Errorf("set failed: %s", string(c)) }
        fmt.Println("ok")
        return nil
    }
    assigns.AddCommand(assignsSet)
    root.AddCommand(assigns)

    // approvals CLI
    approvals := &cobra.Command{Use: "approvals", Short: "Approvals management"}
    var apiBase, token string
    approvals.PersistentFlags().StringVar(&apiBase, "api", "http://localhost:8080", "Server HTTP address")
    approvals.PersistentFlags().StringVar(&token, "token", os.Getenv("CROUPIER_TOKEN"), "JWT token (or set CROUPIER_TOKEN env)")
    // list
    listApprovalsCmd := &cobra.Command{Use: "list", Short: "List approvals"}
    var state, fid, gid, env string
    var page, size int
    listApprovalsCmd.Flags().StringVar(&state, "state", "pending", "state filter: pending|approved|rejected|(empty)")
    listApprovalsCmd.Flags().StringVar(&fid, "function_id", "", "function id filter")
    listApprovalsCmd.Flags().StringVar(&gid, "game_id", "", "game id filter")
    listApprovalsCmd.Flags().StringVar(&env, "env", "", "env filter")
    listApprovalsCmd.Flags().IntVar(&page, "page", 1, "page number")
    listApprovalsCmd.Flags().IntVar(&size, "size", 20, "page size")
    listApprovalsCmd.RunE = func(cmd *cobra.Command, args []string) error {
        qs := make(url.Values)
        if state != "" { qs.Set("state", state) }
        if fid != "" { qs.Set("function_id", fid) }
        if gid != "" { qs.Set("game_id", gid) }
        if env != "" { qs.Set("env", env) }
        qs.Set("page", fmt.Sprintf("%d", page))
        qs.Set("size", fmt.Sprintf("%d", size))
        req, _ := http.NewRequest("GET", apiBase+"/api/approvals?"+qs.Encode(), nil)
        if token != "" { req.Header.Set("Authorization", "Bearer "+token) }
        resp, err := http.DefaultClient.Do(req)
        if err != nil { return err }
        defer resp.Body.Close()
        if resp.StatusCode/100 != 2 { b,_ := io.ReadAll(resp.Body); return fmt.Errorf("list failed: %s", string(b)) }
        io.Copy(os.Stdout, resp.Body)
        return nil
    }
    approvals.AddCommand(listApprovalsCmd)
    // get
    getCmd := &cobra.Command{Use: "get <id>", Short: "Get approval detail"}
    getCmd.RunE = func(cmd *cobra.Command, args []string) error {
        if len(args) == 0 { return fmt.Errorf("id required") }
        req, _ := http.NewRequest("GET", apiBase+"/api/approvals/get?id="+url.QueryEscape(args[0]), nil)
        if token != "" { req.Header.Set("Authorization", "Bearer "+token) }
        resp, err := http.DefaultClient.Do(req)
        if err != nil { return err }
        defer resp.Body.Close()
        if resp.StatusCode/100 != 2 { b,_ := io.ReadAll(resp.Body); return fmt.Errorf("get failed: %s", string(b)) }
        io.Copy(os.Stdout, resp.Body)
        return nil
    }
    approvals.AddCommand(getCmd)
    // approve
    approveCmd := &cobra.Command{Use: "approve <id>", Short: "Approve and execute"}
    var otpCode string
    approveCmd.Flags().StringVar(&otpCode, "otp", "", "TOTP code (if required)")
    approveCmd.RunE = func(cmd *cobra.Command, args []string) error {
        if len(args) == 0 { return fmt.Errorf("id required") }
        body := map[string]any{"id": args[0]}
        if otpCode != "" { body["otp"] = otpCode }
        b,_ := json.Marshal(body)
        req, _ := http.NewRequest("POST", apiBase+"/api/approvals/approve", bytes.NewReader(b))
        req.Header.Set("Content-Type", "application/json")
        if token != "" { req.Header.Set("Authorization", "Bearer "+token) }
        resp, err := http.DefaultClient.Do(req)
        if err != nil { return err }
        defer resp.Body.Close()
        if resp.StatusCode/100 != 2 { c,_ := io.ReadAll(resp.Body); return fmt.Errorf("approve failed: %s", string(c)) }
        io.Copy(os.Stdout, resp.Body)
        return nil
    }
    approvals.AddCommand(approveCmd)
    // reject
    rejectCmd := &cobra.Command{Use: "reject <id>", Short: "Reject with reason"}
    var reason string
    rejectCmd.Flags().StringVar(&reason, "reason", "", "reason message")
    rejectCmd.RunE = func(cmd *cobra.Command, args []string) error {
        if len(args) == 0 { return fmt.Errorf("id required") }
        b,_ := json.Marshal(map[string]any{"id": args[0], "reason": reason})
        req, _ := http.NewRequest("POST", apiBase+"/api/approvals/reject", bytes.NewReader(b))
        req.Header.Set("Content-Type", "application/json")
        if token != "" { req.Header.Set("Authorization", "Bearer "+token) }
        resp, err := http.DefaultClient.Do(req)
        if err != nil { return err }
        defer resp.Body.Close()
        if resp.StatusCode/100 != 2 { c,_ := io.ReadAll(resp.Body); return fmt.Errorf("reject failed: %s", string(c)) }
        io.Copy(os.Stdout, resp.Body)
        return nil
    }
    approvals.AddCommand(rejectCmd)
    root.AddCommand(approvals)

    if err := root.Execute(); err != nil { log.Fatal(err) }
}

// execCommand wraps exec.Command for easier testing.
func execCommand(path string) *exec.Cmd {
    return exec.Command(path)
}
