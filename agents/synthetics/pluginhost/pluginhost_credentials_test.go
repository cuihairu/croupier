//go:build !no_go_plugin

package pluginhost

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/cuihairu/croupier/agents/synthetics/config"
)

const credEnv = "ECHO_PROBE_TEST_TOKEN"

// TestControllerResolvesSecretEnvAndDeliversCredentials proves the
// boundary's positive path over the real go-plugin subprocess: config
// carries only the env var NAME, the controller resolves the value from
// its own environment at check time, and the reference plugin receives
// it on the Check call (the cred-target detail reports the count).
func TestControllerResolvesSecretEnvAndDeliversCredentials(t *testing.T) {
	t.Setenv(credEnv, "tok-s3cr3t-123")
	c := echoController(t, echoprobeBin, 10*time.Second)
	out := c.Check(context.Background(), config.Target{
		Name: "cred-target", Plugin: "echo", Timeout: 5 * time.Second, SecretEnv: []string{credEnv},
	})
	if out.Cause != "" {
		t.Fatalf("unexpected cause %q", out.Cause)
	}
	if !strings.Contains(out.Detail, "1 credential(s)") {
		t.Fatalf("plugin did not receive the credential: detail=%q", out.Detail)
	}
	if strings.Contains(out.Detail, "tok-s3cr3t-123") || strings.Contains(out.Detail, credEnv) {
		t.Fatalf("detail must carry neither value nor name: %q", out.Detail)
	}
}

// TestControllerMissingSecretEnvDegradesBeforeLaunch: an unset (or
// empty) reference degrades with plugin_credential_missing naming the
// variable but never a value, and no plugin subprocess is started for it.
func TestControllerMissingSecretEnvDegradesBeforeLaunch(t *testing.T) {
	// Case 1: the env var is entirely unset in the host process.
	// Case 2: it is set but empty — equally unusable as a credential.
	for name, setup := range map[string]func(t *testing.T){
		"unset": func(t *testing.T) { t.Setenv("CROUPIER_TEST_MISSING_SECRET_IS_SET", "1") },
		"empty": func(t *testing.T) { t.Setenv("CROUPIER_TEST_MISSING_SECRET", "") },
	} {
		t.Run(name, func(t *testing.T) {
			setup(t)
			envName := "CROUPIER_TEST_MISSING_SECRET"
			if name == "unset" {
				envName = "CROUPIER_TEST_MISSING_SECRET_ALSO_UNSET"
			}
			c := echoController(t, echoprobeBin, 10*time.Second)
			out := c.Check(context.Background(), config.Target{
				Name: "cred-target", Plugin: "echo", Timeout: 5 * time.Second,
				SecretEnv: []string{envName},
			})
			if !strings.HasPrefix(out.Cause, "plugin_credential_missing") || !strings.Contains(out.Cause, envName) {
				t.Fatalf("cause=%q want plugin_credential_missing naming the env var", out.Cause)
			}
			if out.Result.Target != "" || out.Result.OK {
				t.Fatalf("degraded result must be zero: %+v", out.Result)
			}
			if c.Version("echo") != "" {
				t.Fatal("credential resolution failure must not launch the plugin")
			}
		})
	}
}

// TestControllerRedactsChattyPluginLeak is the core boundary proof over
// the real subprocess: the leak-target reference plugin echoes the
// credential value back in error AND detail, and the host scrubs it from
// the mapped result before it can reach probe_events — while keeping the
// plugin's business failure a business failure (no degradation cause).
func TestControllerRedactsChattyPluginLeak(t *testing.T) {
	secret := "tok-s3cr3t-123"
	t.Setenv(credEnv, secret)
	c := echoController(t, echoprobeBin, 10*time.Second)
	out := c.Check(context.Background(), config.Target{
		Name: "leak-target", Plugin: "echo", Timeout: 5 * time.Second, SecretEnv: []string{credEnv},
	})
	if out.Cause != "" {
		t.Fatalf("a plugin-echoed secret must not degrade the check: %q", out.Cause)
	}
	if out.Result.OK {
		t.Fatalf("leak-target is a business failure: %+v", out.Result)
	}
	for _, got := range []string{out.Result.Error, out.Detail} {
		if strings.Contains(got, secret) {
			t.Fatalf("credential value leaked through: %q", got)
		}
		if !strings.Contains(got, "[redacted]") {
			t.Fatalf("expected scrubbed marker: %q", got)
		}
	}
}

// TestControllerCredentialsNeverRideDescribe: the Describe handshake is
// issued before any target exists, so its metadata can only carry
// name/version — asserted by the version stamp staying clean even when
// the plugin was started by a credentialed target.
func TestControllerCredentialsNeverRideDescribe(t *testing.T) {
	t.Setenv(credEnv, "tok-s3cr3t-123")
	c := echoController(t, echoprobeBin, 10*time.Second)
	out := c.Check(context.Background(), config.Target{
		Name: "cred-target", Plugin: "echo", Timeout: 5 * time.Second, SecretEnv: []string{credEnv},
	})
	if out.Cause != "" {
		t.Fatalf("unexpected cause %q", out.Cause)
	}
	if v := c.Version("echo"); v != "1.0.0" {
		t.Fatalf("describe stamp polluted: %q", v)
	}
}

// TestEngineLogCarriesRedactedMessage: call errors pass through the
// shared engine's log and cause strings; a wrapped error must surface
// only scrubbed text there while keeping its classification chain.
func TestEngineLogCarriesRedactedMessage(t *testing.T) {
	var buf bytes.Buffer
	log := slog.New(slog.NewTextHandler(&buf, nil))
	base := errors.New("boom: tok-s3cr3t-123")
	redact := newRedactor(map[string]string{credEnv: "tok-s3cr3t-123"})
	wrapped := &redactedError{msg: redact(base.Error()), err: base}
	log.Warn("plugin call failed; degrading", "error", wrapped.Error())
	if out := buf.String(); strings.Contains(out, "tok-s3cr3t-123") {
		t.Fatalf("engine log carried the credential: %s", out)
	}
	if !errors.Is(wrapped, base) {
		t.Fatal("redacted wrapper must preserve the error chain")
	}
}

// TestNewRedactorScrubbing: longest values replace first so a value that
// prefixes another cannot leave a partial secret behind; no credentials
// means plain passthrough.
func TestNewRedactorScrubbing(t *testing.T) {
	r := newRedactor(map[string]string{"A": "short", "B": "a-longer-secret"})
	if got := r("x a-longer-secret short y"); got != "x [redacted] [redacted] y" {
		t.Fatalf("scrub order: %q", got)
	}
	if got := newRedactor(nil)("untouched tok"); got != "untouched tok" {
		t.Fatalf("identity without credentials: %q", got)
	}
}
