package main

// main() 的错误路径与 parseAggregateKV 边界（happy path 已有
// TestMainEndToEnd 覆盖）。fatalExit 接缝替换为 panic 型 sentinel，
// 使含 os.Exit 的 fatalf 路径可在进程内断言；stdout 注入用只读
// 打开的 os.DevNull（写必报 EBADF，且类型满足 *os.File）。

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/pluginpb"
)

// TestCodeGeneratorResponseMarshalPin pin：CodeGeneratorResponse（文件名 +
// 内容均为 string 字段）Marshal 恒成功——main.go Marshal(resp) 错误分支
// C 类豁免的不变式。
func TestCodeGeneratorResponseMarshalPin(t *testing.T) {
	resp := &pluginpb.CodeGeneratorResponse{
		File: []*pluginpb.CodeGeneratorResponse_File{
			{Name: proto.String("demo/v1/ping.croupier.json"), Content: proto.String("{}")},
		},
	}
	_, err := proto.Marshal(resp)
	require.NoError(t, err)
}

// fatalPanic 把 fatalExit 替换为 panic 型替身，返回断言函数。
func fatalPanic(t *testing.T) func(t *testing.T, want string) {
	t.Helper()
	old := fatalExit
	fatalExit = func(format string, a ...any) {
		panic(fmt.Sprintf(format, a...))
	}
	t.Cleanup(func() { fatalExit = old })
	return func(t *testing.T, want string) {
		t.Helper()
		r := recover()
		require.NotNil(t, r, "expected fatalExit to fire")
		require.Contains(t, r, want)
	}
}

func withStdin(t *testing.T, data []byte) {
	t.Helper()
	stdinR, stdinW, err := os.Pipe()
	require.NoError(t, err)
	_, err = stdinW.Write(data)
	require.NoError(t, err)
	require.NoError(t, stdinW.Close())
	old := os.Stdin
	os.Stdin = stdinR
	t.Cleanup(func() { os.Stdin = old; _ = stdinR.Close() })
}

// withStdinReadError 用目录 fd 作 stdin：对目录 Read 恒报 EISDIR。
func withStdinReadError(t *testing.T) {
	t.Helper()
	dirFile, err := os.Open(os.TempDir())
	require.NoError(t, err)
	old := os.Stdin
	os.Stdin = dirFile
	t.Cleanup(func() { os.Stdin = old; _ = dirFile.Close() })
}

func TestMainReadStdinError(t *testing.T) {
	expect := fatalPanic(t)
	withStdinReadError(t)

	defer expect(t, "read stdin")
	main()
}

func TestMainBadStdinUnmarshal(t *testing.T) {
	expect := fatalPanic(t)
	withStdin(t, []byte{0xff, 0xff, 0xff, 0xff})

	defer expect(t, "unmarshal CodeGeneratorRequest")
	main()
}

func TestMainStdoutWriteError(t *testing.T) {
	expect := fatalPanic(t)
	data, err := proto.Marshal(buildTestRequest())
	require.NoError(t, err)
	withStdin(t, data)

	roFile, err := os.OpenFile(os.DevNull, os.O_RDONLY, 0)
	require.NoError(t, err)
	oldOut := os.Stdout
	os.Stdout = roFile
	t.Cleanup(func() { os.Stdout = oldOut; _ = roFile.Close() })

	defer expect(t, "write stdout")
	main()
}

func TestParseAggregateKVLeadingSeparators(t *testing.T) {
	// 前导分隔符：由循环顶部跳过段消费（值后收尾循环只管尾随分隔符，
	// 循环顶部的跳过段只有畸形前导输入才能命中）。
	got := parseAggregateKV(`{, a: 1}`)
	require.Equal(t, map[string]string{"a": "1"}, got)
}

func TestParseAggregateKVOnlySeparators(t *testing.T) {
	// 全分隔符：跳过段消费到 i>=len(src)，触发跳过段后的 break。
	require.Empty(t, parseAggregateKV(`{,,}`))
}

func TestParseAggregateKVSpaceBeforeColon(t *testing.T) {
	// 字段名与冒号之间的空格由 skip-to-colon 段消费。
	got := parseAggregateKV(`{a : 1}`)
	require.Equal(t, map[string]string{"a": "1"}, got)
}

func TestParseAggregateKVTrailingSeparator(t *testing.T) {
	// 尾随分隔符由值解析后的收尾跳过段消费，正确忽略。
	got := parseAggregateKV(`{risk: "high",}`)
	require.Equal(t, map[string]string{"risk": "high"}, got)
}

func TestParseAggregateKVValuelessField(t *testing.T) {
	// 有字段名无冒号无值：skip-to-colon 走完 i==len，值段前 break。
	got := parseAggregateKV(`{risk}`)
	require.Empty(t, got)
}

func TestParseAggregateKVMixedValidAndValueless(t *testing.T) {
	// 有效字段在前、无值字段在后：无值字段丢弃，有效字段保留。
	got := parseAggregateKV(`{risk: "low", orphan}`)
	require.Equal(t, map[string]string{"risk": "low"}, got)
}
