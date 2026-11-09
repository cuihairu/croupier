package cdc

// 契约面补测：Operation/Position 闭集校验、Position.String/ParsePosition
// 往返与畸形串、Capabilities 的 Has/Enabled/Requires/Validate 全臂、
// ParseCapabilities 的严格解析（坏 JSON 与未知键拒绝）、ErrUnsupportedSnapshot
// 的 errors.Is 判定（直接/包装/异类）、ChangeEvent 的 QualifiedTable 与
// Validate 全守卫臂。
//
// 诚实跳过：无——全部为纯函数/纯条件分支，可直测。

import (
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOperationValid(t *testing.T) {
	require.True(t, OpInsert.Valid())
	require.True(t, OpUpdate.Valid())
	require.True(t, OpDelete.Valid())
	require.False(t, Operation("truncate").Valid())
	require.False(t, Operation("").Valid())
}

func TestPositionValidAndString(t *testing.T) {
	for _, kind := range []string{"gtid", "lsn", "scn", "token"} {
		require.True(t, Position{Kind: kind}.Valid())
	}
	require.False(t, Position{}.Valid())
	require.False(t, Position{Kind: "binlog-off"}.Valid())

	require.Empty(t, Position{}.String())
	require.Equal(t, "gtid:uuid-set", Position{Kind: "gtid", Value: "uuid-set"}.String())
}

func TestParsePositionRoundTripAndErrors(t *testing.T) {
	// 空串 = 零位点
	p, err := ParsePosition("")
	require.NoError(t, err)
	require.Equal(t, Position{}, p)

	p, err = ParsePosition("lsn:1042")
	require.NoError(t, err)
	require.Equal(t, Position{Kind: "lsn", Value: "1042"}, p)
	require.Equal(t, "lsn:1042", p.String())

	// 无冒号 / 空 kind / 未知 kind
	_, err = ParsePosition("noseparator")
	require.ErrorContains(t, err, `malformed position`)
	_, err = ParsePosition(":1042")
	require.ErrorContains(t, err, `malformed position`)
	_, err = ParsePosition("binlog-off:42")
	require.ErrorContains(t, err, `unknown position kind`)
}

func TestCapabilitiesHasEnabled(t *testing.T) {
	var zero Capabilities
	for _, cap := range AllCapabilities() {
		require.False(t, zero.Has(cap))
	}
	require.False(t, zero.Has(Capability("unknown")))

	full := Capabilities{
		BeforeImage: true, AfterImage: true, TransactionMetadata: true,
		Snapshot: true, ResumableCheckpoint: true, Pushdown: true, SchemaHistory: true,
	}
	require.Len(t, full.Enabled(), len(AllCapabilities()))

	sub := Capabilities{BeforeImage: true, Pushdown: true}
	require.Equal(t, []Capability{CapBeforeImage, CapPushdown}, sub.Enabled())
	require.Empty(t, Capabilities{}.Enabled())
}

func TestCapabilitiesRequires(t *testing.T) {
	src := Capabilities{BeforeImage: true}

	require.NoError(t, src.Requires())
	require.NoError(t, src.Requires(CapBeforeImage))

	err := src.Requires(CapAfterImage)
	require.ErrorContains(t, err, "afterImage")

	err = src.Requires(CapAfterImage, CapSnapshot)
	require.ErrorContains(t, err, "afterImage, snapshot")
}

func TestCapabilitiesValidate(t *testing.T) {
	require.ErrorContains(t,
		(Capabilities{Snapshot: true}).Validate(), "snapshot without resumableCheckpoint")
	require.NoError(t, (Capabilities{Snapshot: true, ResumableCheckpoint: true}).Validate())
	require.NoError(t, (Capabilities{ResumableCheckpoint: true}).Validate())
}

func TestParseCapabilitiesStrict(t *testing.T) {
	c, err := ParseCapabilities([]byte(`{"beforeImage":true,"pushdown":true}`))
	require.NoError(t, err)
	require.True(t, c.Has(CapBeforeImage))
	require.True(t, c.Has(CapPushdown))
	require.False(t, c.Has(CapSnapshot))

	// 坏 JSON
	_, err = ParseCapabilities([]byte(`{`))
	require.ErrorContains(t, err, "parse capabilities")

	// 未知能力键拒绝（闭集契约）
	_, err = ParseCapabilities([]byte(`{"beforeImage":true,"futureCap":true}`))
	require.ErrorContains(t, err, "unknown field")
}

func TestErrUnsupportedSnapshotIs(t *testing.T) {
	require.True(t, errors.Is(ErrUnsupportedSnapshot, ErrUnsupportedSnapshot))
	require.True(t, errors.Is(fmt.Errorf("open: %w", ErrUnsupportedSnapshot), ErrUnsupportedSnapshot))
	require.False(t, errors.Is(errors.New("other"), ErrUnsupportedSnapshot))
	require.Contains(t, ErrUnsupportedSnapshot.Error(), "does not support snapshot")

	// errors.Is 对相等情形短路、对 %w 包装走 fmt.wrapError 自带的 Is——
	// 契约方法本体只能直测两臂。
	require.True(t, errUnsupportedSnapshot{}.Is(ErrUnsupportedSnapshot))
	require.False(t, errUnsupportedSnapshot{}.Is(errors.New("other")))
}

func TestChangeEventValidate(t *testing.T) {
	valid := ChangeEvent{
		SourceID: "src", SourceType: "mysql", Database: "app", Table: "rows",
		Operation: OpInsert, Position: Position{Kind: "gtid", Value: "x"},
		TsUnixMs: 1730000000000,
	}
	require.Equal(t, "app.rows", valid.QualifiedTable())

	// 各守卫臂
	noSource := valid
	noSource.SourceID = ""
	require.ErrorContains(t, noSource.Validate(), "missing source identity")

	noTable := valid
	noTable.Table = ""
	require.ErrorContains(t, noTable.Validate(), "missing table identity")

	badOp := valid
	badOp.Operation = "truncate"
	require.ErrorContains(t, badOp.Validate(), "unknown operation")

	badPos := valid
	badPos.Position = Position{}
	require.ErrorContains(t, badPos.Validate(), "position kind")

	noTs := valid
	noTs.TsUnixMs = 0
	require.ErrorContains(t, noTs.Validate(), "timestamp missing")

	// update 无双像：契约层拒绝（能力不足 nil 不适用 update）
	imgless := valid
	imgless.Operation = OpUpdate
	require.ErrorContains(t, imgless.Validate(), "neither before nor after image")

	// insert 无像合法（能力不足 nil）；update 单像合法
	require.NoError(t, valid.Validate())

	upd := valid
	upd.Operation = OpUpdate
	upd.After = map[string]json.RawMessage{"id": []byte("1")}
	require.NoError(t, upd.Validate())
}
