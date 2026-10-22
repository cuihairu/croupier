package agent

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
	"time"

	processcore "github.com/cuihairu/croupier/core/process"
	"github.com/stretchr/testify/assert"
)

func TestSupervisorStateIsCoreAlias(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "ops_supervisor_state.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}

	aliasFound := false
	ast.Inspect(file, func(node ast.Node) bool {
		decl, ok := node.(*ast.TypeSpec)
		if !ok || decl.Name.Name != "supervisorState" {
			return true
		}
		alias, ok := decl.Type.(*ast.SelectorExpr)
		if ok {
			ident, _ := alias.X.(*ast.Ident)
			aliasFound = ident != nil && ident.Name == "processcore" && alias.Sel.Name == "SupervisorState"
		}
		return false
	})
	assert.True(t, aliasFound, "supervisorState must remain an alias of core/process.SupervisorState")
}

func TestSupervisorStateAliasForwardsConstruction(t *testing.T) {
	config := ManagedProcessConfig{
		RestartBackoffInitial: 10 * time.Millisecond,
		RestartBackoffMax:     20 * time.Millisecond,
		RestartBreakerLimit:   2,
	}

	state := newSupervisorState(config)
	assert.IsType(t, &processcore.SupervisorState{}, state)
	assert.Equal(t, processcore.RestartConfig{
		InitialBackoff: config.RestartBackoffInitial,
		MaxBackoff:     config.RestartBackoffMax,
		BreakerLimit:   config.RestartBreakerLimit,
	}, state.Config())

	now := time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)
	decision := state.DecideExit(true, now)
	assert.Equal(t, processcore.SupervisorBackoff, decision.Action)
	assert.Equal(t, now.Add(10*time.Millisecond), state.NextRestartAt())
}

func TestSupervisorStateBackoffConsumedOnce(t *testing.T) {
	now := time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)
	state := newSupervisorState(ManagedProcessConfig{
		RestartBackoffInitial: time.Nanosecond,
		RestartBackoffMax:     time.Nanosecond,
	})
	state.DecideExit(true, now)

	assert.True(t, state.ConsumeBackoff(now.Add(time.Nanosecond)))
	assert.False(t, state.ConsumeBackoff(now.Add(time.Nanosecond)))
	assert.Equal(t, 1, state.ConsecutiveFails())
}
