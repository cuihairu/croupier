package agent

import (
	"testing"
)

func TestStartLocalServerWiresLocalHandlerToUpstream(t *testing.T) {
	app := NewWithConfigDir("", "agent-1", t.TempDir())
	app.SetLocalAddr("127.0.0.1:0")
	t.Cleanup(app.Stop)

	if err := app.StartLocalServer(); err != nil {
		t.Fatalf("StartLocalServer() error = %v", err)
	}

	if app.localHandler == nil {
		t.Fatal("expected local handler to be initialized")
	}
	if app.upstream == nil {
		t.Fatal("expected upstream client to be initialized")
	}
	if app.upstream.localHandler != app.localHandler {
		t.Fatal("expected upstream to use the local handler for bidirectional session requests")
	}
	if app.GetLocalServerAddr() == "" {
		t.Fatal("expected local server address")
	}
}
