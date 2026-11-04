// 覆盖目标：VerifyChecksum 读侧校验原语——用真实 file 对象存储重读对象并
// 重算 sha256，与记录校验值比对；不一致/缺字段/读失败各分支。
package backupexec

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/objstore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newVerifyFileStore(t *testing.T) objstore.Store {
	t.Helper()
	st, err := objstore.OpenFile(context.Background(), objstore.Config{BaseDir: t.TempDir()})
	require.NoError(t, err)
	return st
}

func checksumOf(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// TestVerifyChecksum_Matched 重读对象 → 重算 sha256 与记录校验值一致。
func TestVerifyChecksum_Matched(t *testing.T) {
	ctx := context.Background()
	st := newVerifyFileStore(t)
	payload := []byte("SELECT 1;\n-- verify matched\n")
	key := "backups/verify-demo.sql"

	require.NoError(t, st.Put(ctx, key, strings.NewReader(string(payload)), int64(len(payload)), "application/sql"))

	matched, err := VerifyChecksum(ctx, st, &model.Backup{
		BackupID: "verify-1",
		Location: key,
		Checksum: checksumOf(payload),
	})
	require.NoError(t, err)
	assert.True(t, matched)
}

// TestVerifyChecksum_Mismatch 重读对象后重算值 ≠ 记录校验值 → (false, nil)。
func TestVerifyChecksum_Mismatch(t *testing.T) {
	ctx := context.Background()
	st := newVerifyFileStore(t)
	payload := []byte("SELECT 1;\n-- verify mismatch\n")
	key := "backups/verify-demo.sql"

	require.NoError(t, st.Put(ctx, key, strings.NewReader(string(payload)), int64(len(payload)), "application/sql"))

	matched, err := VerifyChecksum(ctx, st, &model.Backup{
		BackupID: "verify-2",
		Location: key,
		Checksum: "deadbeef", // 与真实值不符
	})
	require.NoError(t, err)
	assert.False(t, matched)
}

// TestVerifyChecksum_FieldErrors 缺 Location/缺 Checksum/缺 store → 错误。
func TestVerifyChecksum_FieldErrors(t *testing.T) {
	ctx := context.Background()
	st := newVerifyFileStore(t)

	_, err := VerifyChecksum(ctx, nil, &model.Backup{BackupID: "v", Location: "k", Checksum: "c"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "object store required")

	_, err = VerifyChecksum(ctx, st, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "backup record required")

	_, err = VerifyChecksum(ctx, st, &model.Backup{BackupID: "v", Checksum: "c"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no stored object location")

	_, err = VerifyChecksum(ctx, st, &model.Backup{BackupID: "v", Location: "k"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no recorded checksum")
}

// TestVerifyChecksum_ReadError 对象存储读失败 → 错误（包装）。
func TestVerifyChecksum_ReadError(t *testing.T) {
	ctx := context.Background()
	f := newFakeObjStore(nil)
	f.getErr = assert.AnError

	_, err := VerifyChecksum(ctx, f, &model.Backup{
		BackupID: "verify-3",
		Location: "backups/x.sql",
		Checksum: "abc",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "read backup object")
}

// TestVerifyChecksum_MissingObject 对象不存在（file 驱动 not-exist）→ 错误。
func TestVerifyChecksum_MissingObject(t *testing.T) {
	ctx := context.Background()
	st := newVerifyFileStore(t)

	_, err := VerifyChecksum(ctx, st, &model.Backup{
		BackupID: "verify-4",
		Location: "backups/missing.sql",
		Checksum: "abc",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "read backup object")
}
