package croupier

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// NoOpLogger
// ---------------------------------------------------------------------------

func TestBoostNoOpLogger_Debugf(t *testing.T) {
	l := &NoOpLogger{}
	l.Debugf("should not output %d", 1)
	if GetGlobalLogger() == nil {
		t.Fatal("global logger must never be nil")
	}
}

func TestBoostNoOpLogger_Infof(t *testing.T) {
	l := &NoOpLogger{}
	l.Infof("should not output %s", "x")
}

func TestBoostNoOpLogger_Warnf(t *testing.T) {
	l := &NoOpLogger{}
	l.Warnf("should not output")
}

func TestBoostNoOpLogger_Errorf(t *testing.T) {
	l := &NoOpLogger{}
	l.Errorf("should not output")
}

func TestBoostNoOpLogger_SatisfiesInterface(t *testing.T) {
	var _ Logger = &NoOpLogger{}
	var _ Logger = NewDefaultLogger(false, nil)
}

// ---------------------------------------------------------------------------
// DefaultLogger
// ---------------------------------------------------------------------------

func TestBoostDefaultLogger_DebugDisabled(t *testing.T) {
	var buf bytes.Buffer
	l := NewDefaultLogger(false, &buf)
	l.Debugf("hidden %d", 42)
	if buf.Len() != 0 {
		t.Fatalf("debug output should be suppressed, got %q", buf.String())
	}
}

func TestBoostDefaultLogger_DebugEnabled(t *testing.T) {
	var buf bytes.Buffer
	l := NewDefaultLogger(true, &buf)
	l.Debugf("visible %d", 42)
	if !strings.Contains(buf.String(), "[DEBUG] visible 42") {
		t.Fatalf("unexpected debug output %q", buf.String())
	}
}

func TestBoostDefaultLogger_Infof(t *testing.T) {
	var buf bytes.Buffer
	l := NewDefaultLogger(false, &buf)
	l.Infof("hello %s", "world")
	if !strings.Contains(buf.String(), "[INFO] hello world") {
		t.Fatalf("unexpected info output %q", buf.String())
	}
}

func TestBoostDefaultLogger_Warnf(t *testing.T) {
	var buf bytes.Buffer
	l := NewDefaultLogger(false, &buf)
	l.Warnf("careful %s", "!")
	if !strings.Contains(buf.String(), "[WARN] careful !") {
		t.Fatalf("unexpected warn output %q", buf.String())
	}
}

func TestBoostDefaultLogger_Errorf(t *testing.T) {
	var buf bytes.Buffer
	l := NewDefaultLogger(false, &buf)
	l.Errorf("boom %s", "!")
	if !strings.Contains(buf.String(), "[ERROR] boom !") {
		t.Fatalf("unexpected error output %q", buf.String())
	}
}

func TestBoostDefaultLogger_NilWriterDefaultsToStdout(t *testing.T) {
	l := NewDefaultLogger(false, nil)
	if l.out == nil {
		t.Fatal("nil writer should default to stdout")
	}
	l.Infof("written to stdout, safe")
}

// ---------------------------------------------------------------------------
// Global logger helpers
// ---------------------------------------------------------------------------

func TestBoostGlobalLogger_SetGetRoundTrip(t *testing.T) {
	original := GetGlobalLogger()
	defer SetGlobalLogger(original)

	var buf bytes.Buffer
	custom := NewDefaultLogger(true, &buf)
	SetGlobalLogger(custom)
	if GetGlobalLogger() != custom {
		t.Fatal("GetGlobalLogger should return the logger previously set")
	}
}

func TestBoostGlobalLogger_HelpersRouteToCustomLogger(t *testing.T) {
	original := GetGlobalLogger()
	defer SetGlobalLogger(original)

	var buf bytes.Buffer
	SetGlobalLogger(NewDefaultLogger(true, &buf))

	logDebugf("dbg %d", 1)
	logInfof("inf %d", 2)
	logWarnf("wrn %d", 3)
	logErrorf("err %d", 4)

	out := buf.String()
	for _, prefix := range []string{"[DEBUG] dbg 1", "[INFO] inf 2", "[WARN] wrn 3", "[ERROR] err 4"} {
		if !strings.Contains(out, prefix) {
			t.Fatalf("missing %q in %q", prefix, out)
		}
	}
}

func TestBoostGlobalLogger_SetAndGetConcurrent(t *testing.T) {
	original := GetGlobalLogger()
	defer SetGlobalLogger(original)

	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 50; i++ {
			SetGlobalLogger(NewDefaultLogger(false, nil))
			_ = GetGlobalLogger()
		}
	}()
	for i := 0; i < 50; i++ {
		GetGlobalLogger().Infof("concurrent %d", i)
	}
	<-done
}

// ---------------------------------------------------------------------------
// firstNonEmpty / firstNonEmptySlice edge branches
// ---------------------------------------------------------------------------

func TestBoostFirstNonEmpty_AllBranches(t *testing.T) {
	cases := []struct {
		values []string
		want   string
	}{
		{[]string{}, ""},
		{[]string{""}, ""},
		{[]string{"", ""}, ""},
		{[]string{"a"}, "a"},
		{[]string{"", "b"}, "b"},
		{[]string{"", "", "c"}, "c"},
		{[]string{"a", "b", "c"}, "a"},
	}
	for _, tc := range cases {
		if got := firstNonEmpty(tc.values...); got != tc.want {
			t.Fatalf("firstNonEmpty(%v) = %q, want %q", tc.values, got, tc.want)
		}
	}
}

// ---------------------------------------------------------------------------
// nextBackoffDelay floor/cap invariants
// ---------------------------------------------------------------------------

func TestBoostNextBackoffDelay_AlwaysWithinBounds(t *testing.T) {
	c := &client{}
	rc := &ReconnectConfig{
		BackoffMultiplier: 2.0,
		JitterFactor:      0.3,
	}
	const max = 10 * time.Second
	current := time.Millisecond
	for i := 0; i < 20; i++ {
		next := c.nextBackoffDelay(current, max, rc)
		if next < time.Millisecond {
			t.Fatalf("delay %v below 1ms floor", next)
		}
		if next > max+time.Duration(0.3*float64(max)) {
			t.Fatalf("delay %v exceeds max+jitter bound", next)
		}
		current = next
	}
}
