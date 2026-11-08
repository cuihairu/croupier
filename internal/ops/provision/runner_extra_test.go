package provision

// runner.go TfexecRunner 补测：不依赖真实 terraform 二进制的错误路径——
// workDir 不存在的 newTF 早退（Init/Apply/Destroy 三态）、Apply 的变量
// 装配分支（数字变量坏值先于执行报错、字符串变量 HCL 引号装配）。

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTfexecRunnerNewTFErrorsBeforeExec(t *testing.T) {
	r := NewTfexecRunner("/does/not/exist/tf")
	require.Equal(t, "/does/not/exist/tf", r.ExecPath)

	missing := "no-such-workdir"
	ctx := context.Background()

	require.ErrorContains(t, r.Init(ctx, missing), "new terraform in "+missing)
	_, err := r.Apply(ctx, missing, nil)
	require.ErrorContains(t, err, "new terraform in "+missing)
	require.ErrorContains(t, r.Destroy(ctx, missing), "new terraform in "+missing)
}

func TestTfexecRunnerApplyVarAssignmentBeforeExec(t *testing.T) {
	// workDir 合法（tfexec 构造不校验二进制）→ 先走变量装配，再在执行面失败
	r := NewTfexecRunner("/does/not/exist/tf")
	ctx := context.Background()
	dir := t.TempDir()

	// 数字变量坏值：在调起二进制前即报错
	_, err := r.Apply(ctx, dir, map[string]string{"count": "not-a-number"})
	require.ErrorContains(t, err, "var count")
	require.ErrorContains(t, err, "must be a number")

	// 合法变量（数字裸值 + 字符串 HCL 引号）→ 执行面失败（二进制不存在）
	_, err = r.Apply(ctx, dir, map[string]string{"count": "3", "region": "us-east-1"})
	require.ErrorContains(t, err, "apply in "+dir)
}

func TestTFVarAssignmentShapes(t *testing.T) {
	// 数字变量保持裸 HCL number
	got, err := tfVarAssignment("disk_size", "40")
	require.NoError(t, err)
	require.Equal(t, "disk_size=40", got)

	// 数字变量坏值
	_, err = tfVarAssignment("count", "2x")
	require.Error(t, err)

	// 字符串变量统一 strconv.Quote（多行/# 注释安全）
	got, err = tfVarAssignment("user_data", "line1\n# comment")
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(got, `user_data="`))
	require.NotContains(t, got, "\n")
}
