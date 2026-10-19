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
// It declares one function, plugin.httpget.health, whose params are
// {"path": "/..."}; the plugin issues GET <target><path> against the
// game interface given by -target and translates the answer onto the
// closed-set error envelope:
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
	"time"

	"github.com/cuihairu/croupier/agents/sidecar/funcplugin"
	"github.com/cuihairu/croupier/agents/sidecar/funcplugin/declare"
)

const functionID = "plugin.httpget.health"

// paramSchema / resultSchema are the declared contract of the reference
// adapter: params and results are JSON objects by contract.
const (
	paramSchema  = `{"type":"object","properties":{"path":{"type":"string","pattern":"^/"}},"additionalProperties":false}`
	resultSchema = `{"type":"object","properties":{"status":{"type":"integer"},"body":{"type":"string"}},"required":["status","body"],"additionalProperties":false}`
)

type httpGetPlugin struct {
	client *http.Client
	target string
}

func (p *httpGetPlugin) Describe(context.Context) (string, string, error) {
	return "httpget", "1.0.0", nil
}

func (p *httpGetPlugin) ListFunctions(context.Context) ([]declare.Descriptor, error) {
	descs := []declare.Descriptor{{
		ID:           functionID,
		Summary:      "map an invoke onto GET <target><path> of the game HTTP interface",
		ParamSchema:  paramSchema,
		ResultSchema: resultSchema,
		RiskLevel:    declare.RiskLow,
	}}
	if os.Getenv("HTTPGET_DUPLICATE_FUNCTION") != "" {
		descs = append(descs, descs[0])
	}
	return descs, nil
}

func (p *httpGetPlugin) Invoke(ctx context.Context, in funcplugin.InvokeInput) (funcplugin.InvokeOutput, error) {
	out := funcplugin.InvokeOutput{At: time.Now()}
	if in.FunctionID != functionID {
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
	flag.Parse()
	funcplugin.Serve(&httpGetPlugin{
		client: &http.Client{Timeout: 30 * time.Second},
		target: *target,
	})
}
