package server

// C 批覆盖补齐（control_handler.go）：
//   - SetSDKVersionMinimums：空 map 早退与空白键/值过滤分支；
//   - runContractRemovalSweep：清扫返回错误（Error 日志分支）与
//     finalized>0（Info 日志分支）——通过 registry.SetContractService
//     注入 stub 契约服务（生产由 db 注册面实现）驱动，确定性无时序。

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/stretchr/testify/require"
)

// SetSDKVersionMinimums 边界：空 map 直接返回（不覆盖既有配置）；空白
// 键/值的条目被过滤；合法键归一小写、值去空白。
func TestCoverageC_SetSDKVersionMinimums_Edges(t *testing.T) {
	svc := newTestControlService()

	// 空入参：早退，字段保持 nil
	svc.SetSDKVersionMinimums(nil)
	require.Nil(t, svc.sdkVersionMinimums)
	svc.SetSDKVersionMinimums(map[string]string{})
	require.Nil(t, svc.sdkVersionMinimums)

	// 全空白条目被 continue 过滤；合法条目归一化保留
	svc.SetSDKVersionMinimums(map[string]string{
		"  ": "",        // 键、值均为空白：过滤
		"GO": " 0.4.0 ", // 键小写、值去空白
		"js": "   ",     // 值空白：过滤
	})
	require.Equal(t, map[string]string{"go": "0.4.0"}, svc.sdkVersionMinimums)
}

// stubContractMaterializerC 只实现 FinalizeExpiredContractRemovals 的行为
// 分支，其余方法为签名占位（sweep 路径不会触达）。
type stubContractMaterializerC struct {
	finalized int
	err       error
}

func (s *stubContractMaterializerC) RebuildContractFromFunctionMeta(context.Context, string, string, string, spec.FunctionContractInput) error {
	return nil
}

func (s *stubContractMaterializerC) RemoveFunctionContract(context.Context, string, string, string) (string, error) {
	return "", nil
}

func (s *stubContractMaterializerC) MarkContractRemovalPending(context.Context, string, string, string) error {
	return nil
}

func (s *stubContractMaterializerC) FinalizeExpiredContractRemovals(context.Context, time.Duration) (int, error) {
	return s.finalized, s.err
}

func (s *stubContractMaterializerC) RebuildResourceCapability(context.Context, string, string, string) error {
	return nil
}

func (s *stubContractMaterializerC) RebuildProposalsForResource(context.Context, string, string, string) error {
	return nil
}

func (s *stubContractMaterializerC) RebuildProposalForFunction(context.Context, string, string, string) error {
	return nil
}

func (s *stubContractMaterializerC) RegenerateContractTemplates(context.Context, string, string) error {
	return nil
}

// 清扫返回错误：Error 日志分支（不得 panic——失败仅记录，循环继续）。
func TestCoverageC_RunContractRemovalSweep_SweepError(t *testing.T) {
	svc := newTestControlService()
	svc.registry.SetContractService(&stubContractMaterializerC{err: errors.New("coverage: sweep refused")})

	require.NotPanics(t, func() { svc.runContractRemovalSweep() })
}

// 清扫落地了摘除宽限到期的契约：finalized>0 的 Info 日志分支。
func TestCoverageC_RunContractRemovalSweep_FinalizedPositive(t *testing.T) {
	svc := newTestControlService()
	svc.registry.SetContractService(&stubContractMaterializerC{finalized: 3})

	require.NotPanics(t, func() { svc.runContractRemovalSweep() })
}
