package croupier

import (
	"context"
	"math"
	"math/rand"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

func TestProbeNaNAddResource(t *testing.T) {
	schema := map[string]interface{}{"type": "object", "nan": math.NaN()}
	compiler := jsonschema.NewCompiler()
	compiler.DefaultDraft(jsonschema.Draft7)
	err := compiler.AddResource("schema.json", schema)
	t.Logf("AddResource NaN err: %v", err)
	if err == nil {
		t.Log("NaN AddResource did NOT fail")
	}
}

func TestProbeValidateErrorType(t *testing.T) {
	compiler := jsonschema.NewCompiler()
	compiler.DefaultDraft(jsonschema.Draft7)
	_ = compiler.AddResource("schema.json", map[string]interface{}{"type": "object"})
	sch, _ := compiler.Compile("schema.json")
	err := sch.Validate("string-not-object")
	t.Logf("Validate err type: %T err: %v", err, err)
}

func TestProbeRLimitWrite(t *testing.T) {
	dir := t.TempDir()
	var old syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_FSIZE, &old); err != nil {
		t.Skipf("getrlimit: %v", err)
	}
	t.Logf("old limit: %+v", old)
	limit := syscall.Rlimit{Cur: 0, Max: old.Max}
	if err := syscall.Setrlimit(syscall.RLIMIT_FSIZE, &limit); err != nil {
		t.Skipf("setrlimit: %v", err)
	}
	restore := func() {
		_ = syscall.Setrlimit(syscall.RLIMIT_FSIZE, &old)
	}
	defer restore()
	err := atomicWriteFile(dir+"/probe.txt", []byte("hello"))
	restore()
	if err == nil {
		t.Log("atomicWriteFile unexpectedly succeeded under rlimit 0")
	}
}

func TestProbeConcurrentConnect(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			c.Close()
		}
	}()
	inv := newTCPInvoker(&InvokerConfig{Address: ln.Addr().String(), Insecure: true}).(*tcpInvoker)
	defer inv.Close()
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if rand.Intn(2) == 0 {
				time.Sleep(time.Millisecond)
			}
			_ = inv.connect(context.Background())
		}()
	}
	wg.Wait()
}

func TestProbeHTTPNaNInvoke(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"result":{}}`))
	}))
	defer srv.Close()
	inv := NewHTTPInvoker(&InvokerConfig{Address: srv.URL, Insecure: true})
	defer inv.Close()
	_ = inv.SetSchema("fn.nan", map[string]interface{}{"type": "object", "x": math.NaN()})
	_, err := inv.Invoke(context.Background(), "fn.nan", `{}`, InvokeOptions{})
	t.Logf("invoke with NaN schema: %v", err)
}
