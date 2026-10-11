package runner

import (
	"testing"

	"github.com/cuihairu/croupier/agents/capture/config"
	"github.com/cuihairu/croupier/agents/capture/gate"
)

// pinning 测试：buildGateSet 的校验语义必须等于 config.ValidateGates
// （中心面板 dry-run 的校验真值）。任何一侧单独改动（例如新增构造期
// 检查只改 buildGateSet）都会在文本比对上炸，防止「面板校验通过、
// agent 拒绝生效」的静默漂移。

func pinningCases() []struct {
	name string
	g    config.Gates
} {
	base := func() config.Gates {
		var g config.Gates
		g.Reconcile.Rules = []gate.ReconcileRule{
			{Name: "item-ledger", ItemTable: "game.item", LedgerTable: "game.ledger",
				ItemKey: "1", LedgerKey: "0", WindowSec: 60},
		}
		g.CEP.OffHours = []gate.OffHoursRule{
			{Name: "wallet-night", Table: "game.wallet",
				Windows: []gate.OffHoursWindow{{StartMin: 120, EndMin: 240}}},
		}
		g.Lineage.Rules = []gate.LineageRule{
			{Name: "wallet-allow", Table: "game.wallet", Ops: []string{"update"},
				AllowAccounts: []string{"app"}},
		}
		g.Event.Ops = []string{"insert"}
		return g
	}

	cases := []struct {
		name string
		g    config.Gates
	}{
		{"valid", base()},
		{"empty", config.Gates{}},
		{"reconcile-table-bad", func() config.Gates {
			g := base()
			g.Reconcile.Rules[0].ItemTable = "bad"
			return g
		}()},
		{"reconcile-duplicate-routing", func() config.Gates {
			g := base()
			g.Reconcile.Rules = append(g.Reconcile.Rules, g.Reconcile.Rules[0])
			g.Reconcile.Rules[1].Name = "item-ledger-2"
			return g
		}()},
		{"cep-windows-empty", func() config.Gates {
			g := base()
			g.CEP.OffHours[0].Windows = nil
			return g
		}()},
		{"lineage-accounts-empty", func() config.Gates {
			g := base()
			g.Lineage.Rules[0].AllowAccounts = nil
			return g
		}()},
		{"event-ops-unknown", func() config.Gates {
			g := base()
			g.Event.Ops = []string{"truncate"}
			return g
		}()},
	}
	return cases
}

func TestBuildGateSetValidationMirrorsConfigValidate(t *testing.T) {
	for _, tc := range pinningCases() {
		t.Run(tc.name, func(t *testing.T) {
			_, buildErr := buildGateSet(tc.g)
			validateErr := config.ValidateGates(tc.g)
			if (buildErr == nil) != (validateErr == nil) {
				t.Fatalf("error presence mismatch: buildGateSet=%v ValidateGates=%v",
					buildErr, validateErr)
			}
			if buildErr != nil && buildErr.Error() != validateErr.Error() {
				t.Fatalf("error text drift:\nbuildGateSet: %s\nValidateGates: %s",
					buildErr, validateErr)
			}
		})
	}
}
