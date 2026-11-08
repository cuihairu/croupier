package vault

// persist 落盘失败路径：OpenFile 失败（path+".tmp" 被同名目录占位）必须
// 报错并回滚内存态——Write 删除新条目、Delete 恢复被删条目，存储与
// 内存保持一致。write/flush/close/rename 故障需真实 I/O 注入，属防御
// 分支，不在此处凑。

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLocalProvider_WritePersistFailureRollsBack(t *testing.T) {
	t.Setenv("VAULT_TEST_PASS", "persist-pass")
	path := filepath.Join(t.TempDir(), "vault.json")
	p := localTestProvider(t, path, 0, nil)
	unlockOK(t, p)
	if err := p.Write(context.Background(), "a", "1"); err != nil {
		t.Fatalf("seed write: %v", err)
	}

	// path+".tmp" 被目录占位 → persist 的 OpenFile 必然失败。
	if err := os.Mkdir(path+".tmp", 0o700); err != nil {
		t.Fatalf("mkdir tmp: %v", err)
	}
	err := p.Write(context.Background(), "b", "2")
	if err == nil || !strings.Contains(err.Error(), "vault store write") {
		t.Fatalf("Write err = %v, want vault store write failure", err)
	}
	metas, err := p.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(metas) != 1 || metas[0].Path != "a" {
		t.Fatalf("after failed Write entries = %v, want only [a]", metas)
	}
}

func TestLocalProvider_DeletePersistFailureRestores(t *testing.T) {
	t.Setenv("VAULT_TEST_PASS", "persist-pass")
	path := filepath.Join(t.TempDir(), "vault.json")
	p := localTestProvider(t, path, 0, nil)
	unlockOK(t, p)
	if err := p.Write(context.Background(), "a", "1"); err != nil {
		t.Fatalf("seed write: %v", err)
	}

	if err := os.Mkdir(path+".tmp", 0o700); err != nil {
		t.Fatalf("mkdir tmp: %v", err)
	}
	err := p.Delete(context.Background(), "a")
	if err == nil || !strings.Contains(err.Error(), "vault store write") {
		t.Fatalf("Delete err = %v, want vault store write failure", err)
	}
	metas, err := p.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(metas) != 1 || metas[0].Path != "a" {
		t.Fatalf("after failed Delete entries = %v, want [a] restored", metas)
	}
}
