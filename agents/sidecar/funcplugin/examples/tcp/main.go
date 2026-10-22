//go:build !no_go_plugin

// Command tcp is the slice-4 reference external function adapter plugin
// (todo.md:38): a minimal resident program speaking the Croupier sidecar
// function go-plugin contract and mapping Invoke calls onto a game's
// existing plain-TCP admin/game interface — the second protocol of the
// "game's existing HTTP/TCP/admin interface" mapping the design doc
// promises, next to the HTTP adapter in examples/httpget.
//
// Build locally, then reference the binary from the sidecar config via
// functions.plugins[].command:
//
//	go build ./agents/sidecar/funcplugin/examples/tcp
//
// Frame convention (one invoke = one connection = one line each way):
//
//	request: <path>[ <compact json body>]\n
//	reply:   one line, newline- or EOF-terminated, capped at 1 MiB
//
// The request is a single line because a game admin console speaks a
// line protocol; the body is compact JSON when the caller passes one.
// One connection per invoke keeps the adapter free of shared socket
// state across concurrent invokes — game admin protocols carry no
// connection identity of their own.
//
// Every declared function issues the frame against the game interface
// given by -target and translates the answer onto the closed-set error
// envelope:
//
//	reply {"ok":true,...}       → ok=true,  result={"path":..,"body":".."}
//	reply {"ok":false,...}      → ok=false, error_code=target_rejected
//	any other non-empty reply   → ok=true,  verbatim reply as body
//	empty reply / closed early  → ok=false, error_code=target_unavailable
//	target timeout (see below)  → ok=false, error_code=target_unavailable
//	never-terminated reply past → ok=false, error_code=bad_response
//	the 1 MiB cap
//	malformed params            → ok=false, error_code=bad_response
//
// A TCP protocol has no status code, so a reply that is not an
// {"ok":...} object is data, not a refusal: the adapter carries the line
// back verbatim and lets the caller read it. The adapter must not invent
// success semantics a status-less protocol never had.
//
// The socket wait is bounded by the earliest of three: context
// cancellation, the host's per-call deadline, and the adapter's own
// -target-timeout. The last one matters — the host deadline always beats
// a plugin-internal timeout of its own making, so without it a wedged
// game interface would degrade as a host-level plugin_timeout instead of
// a business target_unavailable, and the adapter would never be able to
// report "the game interface did not answer in time". -target-timeout is
// the same knob the HTTP adapter carries as its http.Client timeout.
//
// The mapping target is plugin-local config (-target); the plugin holds
// no Croupier credentials and never learns the server address — the
// host gives it nothing but the call.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
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
	pluginNamespace = "plugin.tcp."
	// defaultFunction is declared when no -functions-file is given.
	defaultFunction = "ping"

	// paramSchema / resultSchema are the declared contract of the
	// reference adapter, and they match what Invoke enforces: params and
	// results are JSON objects, path is a non-empty line. The leading
	// slash is deliberately not required — a line protocol is not an
	// HTTP path.
	paramSchema  = `{"type":"object","properties":{"path":{"type":"string","minLength":1},"body":{"type":"object"}},"required":["path"],"additionalProperties":false}`
	resultSchema = `{"type":"object","properties":{"path":{"type":"string"},"body":{"type":"string"}},"required":["path","body"],"additionalProperties":false}`

	// maxReplyBytes caps one reply line: a game interface must not be able
	// to make the adapter carry an unbounded reply into the envelope.
	maxReplyBytes = 1 << 20
	// defaultTargetTimeout is the adapter's own reply wait. It stays below
	// the host's per-call deadline in practice, which is what makes a wedged
	// target a business failure instead of a host-level plugin_timeout.
	defaultTargetTimeout = 10 * time.Second
)

var errReplyTooLarge = errors.New("reply exceeds the cap")

type tcpPlugin struct {
	target string
	// targetTimeout bounds one dial+write+read; see the package comment on
	// why the adapter owns a timeout of its own rather than only the host's
	// call deadline.
	targetTimeout time.Duration
	// functionsFile, when set, lists one function name per line and is
	// re-read on every ListFunctions call, so the adapter declares exactly
	// the commands the game interface advertises right now — which is what
	// the host's registration reconcile loop tracks.
	functionsFile string
}

func (p *tcpPlugin) Describe(context.Context) (string, string, error) {
	return "tcp", "1.0.0", nil
}

func (p *tcpPlugin) ListFunctions(context.Context) ([]declare.Descriptor, error) {
	names, err := p.functionNames()
	if err != nil {
		return nil, err
	}
	descs := make([]declare.Descriptor, 0, len(names))
	for _, name := range names {
		descs = append(descs, declare.Descriptor{
			ID:           pluginNamespace + name,
			Summary:      "map an invoke onto one request line of the game TCP interface",
			ParamSchema:  paramSchema,
			ResultSchema: resultSchema,
			// The reference adapter declares one low-risk command. A plugin
			// author mapping a mutating admin verb raises RiskLevel per
			// descriptor (low|medium|high|danger); the shared metadata
			// builder derives Mode and Idempotent from it.
			RiskLevel: declare.RiskLow,
		})
	}
	return descs, nil
}

// functionNames is the declared function list: the fixed reference
// function by default, or one name per non-blank line of -functions-file,
// re-read per call so a changing game interface is visible without a
// plugin restart. A read failure is returned rather than an empty list —
// an unreadable list must not silently withdraw every function.
func (p *tcpPlugin) functionNames() ([]string, error) {
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

func (p *tcpPlugin) Invoke(ctx context.Context, in funcplugin.InvokeInput) (funcplugin.InvokeOutput, error) {
	out := funcplugin.InvokeOutput{At: time.Now()}
	if !strings.HasPrefix(in.FunctionID, pluginNamespace) {
		return p.fail(out, "bad_response", fmt.Sprintf("unknown function %q", in.FunctionID))
	}
	var params struct {
		Path string           `json:"path"`
		Body *json.RawMessage `json:"body"`
	}
	if err := json.Unmarshal([]byte(in.ParamsJSON), &params); err != nil {
		return p.fail(out, "bad_response", fmt.Sprintf("params are not the declared JSON object: %v", err))
	}
	// An empty path is an unsendable request line, not a request to the
	// root: fail loudly instead of guessing a command.
	if params.Path == "" {
		return p.fail(out, "bad_response", `params.path must not be empty`)
	}
	if params.Body != nil {
		if !bytes.HasPrefix(bytes.TrimSpace(*params.Body), []byte("{")) {
			return p.fail(out, "bad_response", "params.body must be a JSON object")
		}
	}

	deadline := socketDeadline(ctx, in.Timeout, p.targetTimeout)
	remaining := time.Until(deadline)
	if remaining <= 0 {
		return p.fail(out, "target_unavailable", "call deadline already reached")
	}
	dialer := net.Dialer{Timeout: remaining}
	conn, err := dialer.Dial("tcp", p.target)
	if err != nil {
		return p.fail(out, "target_unavailable", fmt.Sprintf("game interface unreachable: %v", err))
	}
	defer conn.Close()
	if err := conn.SetDeadline(deadline); err != nil {
		return p.fail(out, "target_unavailable", fmt.Sprintf("arming the call deadline: %v", err))
	}

	frame := params.Path
	if params.Body != nil {
		frame += " " + string(bytes.TrimSpace(*params.Body))
	}
	frame += "\n"
	if _, err := io.WriteString(conn, frame); err != nil {
		return p.fail(out, "target_unavailable", fmt.Sprintf("writing to the game interface: %v", err))
	}

	reply, err := readReplyLine(conn, maxReplyBytes)
	switch {
	case err == errReplyTooLarge:
		return p.fail(out, "bad_response", fmt.Sprintf("game interface reply exceeds %d bytes", maxReplyBytes))
	case err != nil:
		return p.fail(out, "target_unavailable", fmt.Sprintf("reading the game interface: %v", err))
	case reply == "":
		// The peer closed without a line: no answer at all, not an empty
		// answer.
		return p.fail(out, "target_unavailable", "game interface closed without replying")
	}
	return p.reply(out, params.Path, reply)
}

// reply classifies one game-interface reply. Only an {"ok":false} object
// is a refusal; anything else is carried back as data.
func (p *tcpPlugin) reply(out funcplugin.InvokeOutput, path, reply string) (funcplugin.InvokeOutput, error) {
	var obj map[string]any
	if err := json.Unmarshal([]byte(reply), &obj); err == nil {
		if ok, present := obj["ok"]; present {
			if okBool, isBool := ok.(bool); isBool && !okBool {
				reason := ""
				for _, key := range []string{"error", "message"} {
					if s, isString := obj[key].(string); isString && s != "" {
						reason = s
						break
					}
				}
				if reason == "" {
					reason = reply
				}
				return p.fail(out, "target_rejected", fmt.Sprintf("game interface refused: %s", reason))
			}
		}
	}
	out.OK = true
	out.ResultJSON = fmt.Sprintf(`{"path":%q,"body":%q}`, path, reply)
	return out, nil
}

func (p *tcpPlugin) fail(out funcplugin.InvokeOutput, code, msg string) (funcplugin.InvokeOutput, error) {
	out.ErrorCode, out.ErrorMessage = code, msg
	return out, nil
}

// socketDeadline bounds the dial+write+read by the earliest of the
// adapter's own target timeout and the caller's remaining deadline, so a
// slow game interface degrades as a business target_unavailable while a
// cancelled or deadline-expired call is still bounded by the caller.
func socketDeadline(ctx context.Context, callTimeout, targetTimeout time.Duration) time.Time {
	deadline := time.Now().Add(targetTimeout)
	if ctxDeadline, ok := ctx.Deadline(); ok && ctxDeadline.Before(deadline) {
		deadline = ctxDeadline
	} else if !ok && callTimeout > 0 {
		if callDeadline := time.Now().Add(callTimeout); callDeadline.Before(deadline) {
			deadline = callDeadline
		}
	}
	return deadline
}

// readReplyLine reads one newline-terminated reply line from the game
// interface, capped at limit bytes. A reply with no trailing newline but
// an EOF counts as one line; an EOF with nothing read is returned as
// ( "", io.EOF ).
func readReplyLine(r io.Reader, limit int) (string, error) {
	var sb strings.Builder
	buf := make([]byte, 4096)
	for {
		n, err := r.Read(buf)
		if n > 0 {
			if idx := bytes.IndexByte(buf[:n], '\n'); idx >= 0 {
				sb.Write(buf[:idx])
				return strings.TrimRight(sb.String(), "\r"), nil
			}
			if sb.Len()+n > limit {
				return "", errReplyTooLarge
			}
			sb.Write(buf[:n])
		}
		if err == nil && n == 0 {
			err = io.EOF // legal for io.Reader; never spin on it
		}
		if err == io.EOF {
			if sb.Len() == 0 {
				return "", io.EOF
			}
			return strings.TrimRight(sb.String(), "\r"), nil
		}
		if err != nil {
			return "", err
		}
	}
}

func main() {
	target := flag.String("target", "127.0.0.1:8080", "host:port of the game TCP interface to map onto")
	targetTimeout := flag.Duration("target-timeout", defaultTargetTimeout, "how long to wait for one game interface reply before reporting target_unavailable")
	functionsFile := flag.String("functions-file", "", "optional file listing one declared function name per line, re-read on every ListFunctions call")
	flag.Parse()
	funcplugin.Serve(&tcpPlugin{target: *target, targetTimeout: *targetTimeout, functionsFile: *functionsFile})
}
