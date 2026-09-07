package main

import (
	"bytes"
	"encoding/json"
	"net"
	"net/http"
	"os"
	"os/exec"
	"testing"
	"time"
)

const exampleServerAddr = "127.0.0.1:18780"

// exampleMainPortFree 探测示例硬编码的 Server 地址是否可用。
func exampleMainPortFree(t *testing.T) bool {
	t.Helper()
	ln, err := net.Listen("tcp", exampleServerAddr)
	if err != nil {
		return false
	}
	_ = ln.Close()
	return true
}

// main() 的失败路径：runScenario 对不可达端口报错，main 走 log.Fatalf。
// os.Exit 无法在同进程内验证，放到子进程中执行。
func TestExampleMainServerDownExits(t *testing.T) {
	if os.Getenv("EXAMPLE_HTTP_INVOKER_CHILD") == "1" {
		main()
		return
	}
	if !exampleMainPortFree(t) {
		t.Skipf("port %s occupied; cannot exercise dead-server main path", exampleServerAddr)
	}
	cmd := exec.Command(os.Args[0], "-test.run=TestExampleMainServerDownExits")
	cmd.Env = append(os.Environ(), "EXAMPLE_HTTP_INVOKER_CHILD=1")
	out, err := cmd.CombinedOutput()
	if ee, ok := err.(*exec.ExitError); !ok || ee.ExitCode() != 1 {
		t.Fatalf("expected child exit status 1, got %v\n%s", err, out)
	}
	if !bytes.Contains(out, []byte("example failed")) {
		t.Fatalf("unexpected child output:\n%s", out)
	}
}

// main() 的成功路径：在硬编码端口上起一个最小 Server HTTP API。
func TestExampleMainSucceeds(t *testing.T) {
	if !exampleMainPortFree(t) {
		t.Skipf("port %s occupied; cannot exercise happy-path main", exampleServerAddr)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/functions/player.ban/invoke", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"result": map[string]interface{}{"banned": true},
		})
	})
	ln, err := net.Listen("tcp", exampleServerAddr)
	if err != nil {
		t.Skipf("port %s occupied: %v", exampleServerAddr, err)
	}
	server := &http.Server{Handler: mux}
	go func() { _ = server.Serve(ln) }()
	t.Cleanup(func() { _ = server.Close() })

	done := make(chan struct{})
	go func() {
		main()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("main did not complete")
	}
}
