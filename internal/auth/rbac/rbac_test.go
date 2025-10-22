package rbac

import "testing"

func TestPolicy(t *testing.T) {
    p := NewPolicy()
    p.Grant("u", "function:player.ban")
    if !p.Can("u", "function:player.ban") { t.Fatal("expect allowed") }
    if p.Can("u", "job:cancel") { t.Fatal("unexpected allow") }
    p.Grant("u", "*")
    if !p.Can("u", "job:cancel") { t.Fatal("wildcard should allow") }
}

