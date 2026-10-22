//go:build !no_go_plugin

// Command httpget is the slice-1 example external function adapter
// plugin (todo.md:35): a minimal resident program speaking the Croupier
// sidecar function go-plugin contract and mapping Invoke calls onto a
// game's existing HTTP interface via plain GET requests.
//
// Build locally, then reference the binary from the sidecar config via
// functions.plugins[].command:
//
//	go build ./agents/sidecar/funcplugin/examples/httpget
//
// By default it declares one function, plugin.httpget.health; its params
// are {"path": "/..."}. -functions-file instead declares one function per
// line of a file that is re-read on every ListFunctions call, so the
// declared list can change under a running plugin. Every declared
// function issues GET <target><path> against the game interface given by
// -target and translates the answer onto the closed-set error envelope:
//
//	2xx            → ok=true, result={"status":..,"body":".."}
//	4xx            → ok=false, error_code=target_rejected
//	5xx            → ok=false, error_code=target_unavailable
//	dial/timeout   → ok=false, error_code=target_unavailable
//
// The mapping target is plugin-local config (-target); the plugin holds
// no Croupier credentials and never learns the server address — the
// host gives it nothing but the call.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/cuihairu/croupier/agents/sidecar/funcplugin"
	"github.com/cuihairu/croupier/agents/sidecar/funcplugin/declare"
)

const (
	// pluginNamespace is the only id prefix this adapter declares and
	// serves. The host only ever routes ids it registered, so serving the
	// whole namespace is what a changing function list needs.
	pluginNamespace = "plugin.httpget."
	// defaultFunction is declared when no -functions-file is given.
	defaultFunction = "health"

	// paramSchema / resultSchema are the declared contract of the
	// reference adapter: params and results are JSON objects by contract.
	paramSchema  = `{"type":"object","properties":{"path":{"type":"string","pattern":"^/"}},"additionalProperties":false}`
	resultSchema = `{"type":"object","properties":{"status":{"type":"integer"},"body":{"type":"string"}},"required":["status","body"],"additionalProperties":false}`
)

type httpGetPlugin struct {
	client *http.Client
	target string
	// functionsFile, when set, lists one function name per line and is
	// re-read on every ListFunctions call, so the adapter declares exactly
	// the endpoints the game interface advertises right now — which is
	// what the host's registration reconcile loop tracks.
	functionsFile string
}

func (p *httpGetPlugin) Describe(context.Context) (string, string, error) {
	return "httpget", "1.0.0", nil
}

func (p *httpGetPlugin) ListFunctions(context.Context) ([]declare.Descriptor, error) {
	names, err := p.functionNames()
	if err != nil {
		return nil, err
	}
	descs := make([]declare.Descriptor, 0, len(names))
	for _, name := range names {
		descs = append(descs, declare.Descriptor{
			ID:           pluginNamespace + name,
			Summary:      "map an invoke onto GET <target><path> of the game HTTP interface",
			ParamSchema:  paramSchema,
			ResultSchema: resultSchema,
			RiskLevel:    declare.RiskLow,
		})
	}
	if os.Getenv("HTTPGET_DUPLICATE_FUNCTION") != "" && len(descs) > 0 {
		descs = append(descs, descs[0])
	}
	return descs, nil
}

// functionNames is the declared function list: the fixed reference
// function by default, or one name per non-blank line of -functions-file,
// re-read per call so a changing game interface is visible without a
// plugin restart. A read failure is returned rather than an empty list —
// an unreadable list must not silently withdraw every function.
func (p *httpGetPlugin) functionNames() ([]string, error) {
	if p.functionsFile == "" {
		return []string{defaultFunction}, nil
	}
	body, err := os.ReadFile(p.functionsFile)
	if err != nil {
		return nil, fmt.Errorf("reading functions file %q: %w", p.functionsFile, err)
	}
	var names []string
	for _, line := range strings.Split(string(body), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			names = append(names, line)
		}
	}
	return names, nil
}

func (p *httpGetPlugin) Invoke(ctx context.Context, in funcplugin.InvokeInput) (funcplugin.InvokeOutput, error) {
	out := funcplugin.InvokeOutput{At: time.Now()}
	if !strings.HasPrefix(in.FunctionID, pluginNamespace) {
		return p.fail(out, "bad_response", fmt.Sprintf("unknown function %q", in.FunctionID))
	}
	var params struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal([]byte(in.ParamsJSON), &params); err != nil {
		return p.fail(out, "bad_response", fmt.Sprintf("params are not the declared JSON object: %v", err))
	}
	if params.Path == "" {
		params.Path = "/"
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.target+params.Path, nil)
	if err != nil {
		return p.fail(out, "target_unavailable", fmt.Sprintf("target url rejected: %v", err))
	}
	resp, err := p.client.Do(req)
	if err != nil {
		// Connection failure or deadline: the game interface could not be
		// reached in time (ctx cancellation propagates from the host).
		return p.fail(out, "target_unavailable", err.Error())
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return p.fail(out, "bad_response", fmt.Sprintf("reading target body: %v", err))
	}
	out.ResultJSON = fmt.Sprintf(`{"status":%d,"body":%q}`, resp.StatusCode, string(body))
	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		out.OK = true
	case resp.StatusCode < 500:
		out.ErrorCode, out.ErrorMessage = "target_rejected", fmt.Sprintf("game interface answered %d", resp.StatusCode)
	default:
		out.ErrorCode, out.ErrorMessage = "target_unavailable", fmt.Sprintf("game interface answered %d", resp.StatusCode)
	}
	return out, nil
}

func (p *httpGetPlugin) fail(out funcplugin.InvokeOutput, code, msg string) (funcplugin.InvokeOutput, error) {
	out.ErrorCode, out.ErrorMessage = code, msg
	return out, nil
}

func main() {
	target := flag.String("target", "http://127.0.0.1:8080", "base URL of the game HTTP interface to map onto")
	functionsFile := flag.String("functions-file", "", "optional file listing one declared function name per line, re-read on every ListFunctions call")
	flag.Parse()
	funcplugin.Serve(&httpGetPlugin{
		client:        &http.Client{Timeout: 30 * time.Second},
		target:        *target,
		functionsFile: *functionsFile,
	})
}
