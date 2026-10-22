package config

import (
	"fmt"

	"github.com/cuihairu/croupier/agents/capture/filter"
	"github.com/cuihairu/croupier/agents/capture/gate"
)

// ValidateGates gates 规则集全量校验真值（单一实现，防两侧漂移）：
// 静态规则校验（Validate*：表名形态/必填/严重级）+ 构造期检查（New*：
// 对账表路由唯一性等）+ 事件粗过滤合法性。消费方两处：
//   - agent 侧 runner.buildGateSet（启动加载 + 面板下发热生效，keep last-good）；
//   - 中心面板 dry-run（internal/api/ops 的 POST /capture/gates/validate，
//     发布前拦下「agent 必拒」的坏版本，避免发布成功 ≠ 可生效的运维噪声）。
//
// 仅校验不构造运行期闸对象——dry-run 面不产生副作用。
func ValidateGates(g Gates) error {
	if err := gate.ValidateReconcileRules(g.Reconcile.Rules); err != nil {
		return err
	}
	if err := gate.ValidateCEPRules(g.CEP.OffHours, g.CEP.Burst, g.CEP.Probing); err != nil {
		return err
	}
	if err := gate.ValidateLineageRules(g.Lineage.Rules); err != nil {
		return err
	}
	if len(g.Reconcile.Rules) > 0 {
		if _, err := gate.NewReconcile(g.Reconcile.Rules); err != nil {
			return fmt.Errorf("reconcile gate: %w", err)
		}
	}
	if len(g.CEP.OffHours)+len(g.CEP.Burst)+len(g.CEP.Probing) > 0 {
		if _, err := gate.NewCEP(g.CEP.OffHours, g.CEP.Burst, g.CEP.Probing); err != nil {
			return fmt.Errorf("cep gate: %w", err)
		}
	}
	if len(g.Lineage.Rules) > 0 {
		if _, err := gate.NewLineage(g.Lineage.Rules); err != nil {
			return fmt.Errorf("lineage gate: %w", err)
		}
	}
	if len(g.Event.Ops) > 0 {
		if _, err := filter.NewEventFilter(g.Event.Ops); err != nil {
			return fmt.Errorf("event filter: %w", err)
		}
	}
	return nil
}
