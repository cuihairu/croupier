package drivers

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
)

// stubDriver 是注册表测试用的最小实现。
type stubDriver struct{ kind string }

func (s *stubDriver) Kind() string { return s.kind }
func (s *stubDriver) Call(_ context.Context, _ string, _ []byte) ([]byte, error) {
	return []byte(`{}`), nil
}
func (s *stubDriver) Close() error { return nil }

func factoryFor(kind string) Factory {
	return func(Config) (Driver, error) { return &stubDriver{kind: kind}, nil }
}

func TestRegisterAndNew(t *testing.T) {
	Register("openapi-test", factoryFor("openapi-test"))
	d, err := New("openapi-test", Config{Endpoint: "https://x.example"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if d.Kind() != "openapi-test" {
		t.Fatalf("Kind = %q", d.Kind())
	}
	if err := d.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestNewUnknownKind(t *testing.T) {
	if _, err := New("absent-kind", Config{}); !errors.Is(err, ErrUnknownKind) {
		t.Fatalf("err = %v, want ErrUnknownKind", err)
	}
}

func TestRegisterDuplicatePanics(t *testing.T) {
	Register("dup-kind", factoryFor("dup-kind"))
	defer func() {
		r := recover()
		if r == nil || !strings.Contains(fmt.Sprint(r), "registered twice") {
			t.Fatalf("recover = %v, want duplicate panic", r)
		}
	}()
	Register("dup-kind", factoryFor("dup-kind"))
}

func TestRegisterInvalidArgsPanic(t *testing.T) {
	cases := map[string]func(){
		"empty kind":  func() { Register("", factoryFor("x")) },
		"nil factory": func() { Register("nil-factory", nil) },
	}
	for name, fn := range cases {
		func() {
			defer func() {
				if recover() == nil {
					t.Fatalf("%s: expected panic", name)
				}
			}()
			fn()
		}()
	}
}

func TestKindsSortedStable(t *testing.T) {
	Register("zzz-kind", factoryFor("zzz-kind"))
	Register("aaa-kind", factoryFor("aaa-kind"))
	ks := Kinds()
	for i := 1; i < len(ks); i++ {
		if ks[i-1] > ks[i] {
			t.Fatalf("Kinds not sorted: %v", ks)
		}
	}
	found := 0
	for _, k := range ks {
		if k == "zzz-kind" || k == "aaa-kind" {
			found++
		}
	}
	if found != 2 {
		t.Fatalf("Kinds = %v, missing registered entries", ks)
	}
}

func TestContractErrorMessages(t *testing.T) {
	if got := (&NotFoundError{Name: "quicksdk"}).Error(); got != "provider not found: quicksdk" {
		t.Fatalf("NotFoundError = %q", got)
	}
	if got := (&MethodNotSupportedError{Provider: "p", Method: "m"}).Error(); got != `method "m" not supported by provider "p"` {
		t.Fatalf("MethodNotSupportedError = %q", got)
	}
	if got := (&DisabledError{Name: "p"}).Error(); got != `provider "p" is disabled` {
		t.Fatalf("DisabledError = %q", got)
	}
}

func TestRegisterConcurrent(t *testing.T) {
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			Register(fmt.Sprintf("race-kind-%d", i), factoryFor("race"))
		}(i)
	}
	wg.Wait()
	for i := 0; i < 8; i++ {
		if _, err := New(fmt.Sprintf("race-kind-%d", i), Config{}); err != nil {
			t.Fatalf("New race-kind-%d: %v", i, err)
		}
	}
}
