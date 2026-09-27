package main

// #10 回归：openapi-demo-agent -metadata 启动参数解析与 providers.yaml
// metadata 块产出。demo 实例此前不带任何实例元数据，sdk-distribution 页
// 对该 agent 无元数据可看（OPEN-ISSUES #1 同族）。

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseInstanceMetadata(t *testing.T) {
	t.Parallel()
	// 空输入 → nil（不携带元数据）。
	if meta, err := parseInstanceMetadata("   "); err != nil || meta != nil {
		t.Fatalf("blank input = (%v, %v), want (nil, nil)", meta, err)
	}
	// k=v 逗号分隔多项；键值两侧空白修剪。
	meta, err := parseInstanceMetadata("serverId=demo-1, pod = p-7 ,,bad-pair")
	if err == nil {
		t.Fatalf("bad pair should fail parse, got %v", meta)
	}
	meta, err = parseInstanceMetadata("serverId=demo-1, pod=p-7")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if meta["serverId"] != "demo-1" || meta["pod"] != "p-7" || len(meta) != 2 {
		t.Fatalf("meta = %v, want serverId/pod", meta)
	}
	// 无 = 的段报错且指明段内容。
	if _, err := parseInstanceMetadata("novalue"); err == nil || !strings.Contains(err.Error(), "novalue") {
		t.Fatalf("err = %v, want key=value error naming pair", err)
	}
}

func TestYamlMetadataBlock(t *testing.T) {
	t.Parallel()
	if got := yamlMetadataBlock(nil); got != "" {
		t.Fatalf("nil metadata block = %q, want empty", got)
	}
	got := yamlMetadataBlock(map[string]string{"serverId": "demo-1", "pod": "p-7"})
	for _, want := range []string{"metadata:", `"pod": "p-7"`, `"serverId": "demo-1"`} {
		if !strings.Contains(got, want) {
			t.Fatalf("block missing %q:\n%s", want, got)
		}
	}
	// 键字典序：pod 在 serverId 前。
	if strings.Index(got, `"pod"`) > strings.Index(got, `"serverId"`) {
		t.Fatalf("keys not sorted:\n%s", got)
	}
}

func TestRun_AgentPathWritesMetadataBlock(t *testing.T) {
	fake := newFakeAgent(true, nil)
	factory := func(string, string, string) embeddedAgent { return fake }

	dir := t.TempDir()
	cfg := runConfig{
		serverAddr: "127.0.0.1:19090",
		httpAddr:   "127.0.0.1:0",
		gameID:     "demo",
		env:        "qa",
		agentID:    "demo-agent",
		localAddr:  "127.0.0.1:0",
		configDir:  dir,
		metadata:   "serverId=demo-1,pod=p-7",
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_ = runInGoroutine(t, ctx, cfg, factory)

	select {
	case <-fake.entered:
	case <-time.After(testTimeout):
		t.Fatal("agent Run was not started")
	}

	raw, err := os.ReadFile(filepath.Join(dir, "providers.yaml"))
	if err != nil {
		t.Fatalf("read providers.yaml: %v", err)
	}
	yaml := string(raw)
	for _, want := range []string{`"serverId": "demo-1"`, `"pod": "p-7"`} {
		if !strings.Contains(yaml, want) {
			t.Errorf("providers.yaml missing %q:\n%s", want, yaml)
		}
	}
}

func TestRun_MetadataParseFailureAborts(t *testing.T) {
	fake := newFakeAgent(true, nil)
	factory := func(string, string, string) embeddedAgent { return fake }

	cfg := runConfig{
		serverAddr: "127.0.0.1:19090",
		httpAddr:   "127.0.0.1:0",
		gameID:     "demo",
		env:        "qa",
		agentID:    "demo-agent",
		localAddr:  "127.0.0.1:0",
		configDir:  t.TempDir(),
		metadata:   "not-a-pair",
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := run(ctx, cfg, factory)
	if err == nil || !strings.Contains(err.Error(), "-metadata") {
		t.Fatalf("run err = %v, want -metadata parse error", err)
	}
	select {
	case <-fake.entered:
		t.Fatal("agent must not start when -metadata is invalid")
	default:
	}
}
