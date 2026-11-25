package menu

import (
	"testing"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// This file pins the `!check(parent)` defensive branch of filterAccessibleTree
// (the only remaining exemption; see docs/development/coverage-exemptions.md
// for the full argument). That branch is unreachable under the current
// implementation: when check(item)=true and the parent node exists, check has
// already recursed with ok=check(parent) and got true, so the main loop
// calling check(parent) again hits the cache and returns the same final value.
// We do not delete the product code; instead a pin test locks the invariants
// it relies on (cache consistency, false propagation around cycles, and
// ParentID/byID/permission-set immutability during execution) — if a future
// implementation change makes the branch reachable, this file's tests fail and
// remind us to add error-path cases.

// SetLabels always returns nil: json.Marshal of map[string]string always
// succeeds, and the error return value is kept only to match the existing
// model helper signature (self-evident from the function comment in
// internal/model/menu.go). The error exits of `if err := item.SetLabels(...);
// err != nil` in Create/Update are therefore unreachable. This test pins
// "any input (nil / empty / valid) always returns nil"; if SetLabels later
// gains validation (e.g. an empty labels error), this fails, and at that point
// end-to-end cases must be added for the Create/Update error exits.
func TestCoverageFSetLabelsAlwaysSucceeds(t *testing.T) {
	item := &model.MenuItem{}
	require.NoError(t, item.SetLabels(nil))
	require.NoError(t, item.SetLabels(map[string]string{}))
	require.NoError(t, item.SetLabels(map[string]string{"zh-CN": "玩家", "en-US": "Player"}))
	assert.Equal(t, "玩家", item.GetLabels()["zh-CN"])
}

// Reachability argument for filterAccessibleTree 380-381 (`if !check(parent)
// { continue }`): when the main loop reaches 380, check(item)=true must hold
// and the parent node exists in byID. And per check's definition (348-352),
// when the item is visible, permitted, and has a parent, ok=check(parent) —
// that is, check(item)=true already implies check(parent)=true (cache hit), so
// 380's !check(parent) is always false and the continue never runs.
// This test pins the dirty-data semantics the invariant rests on:
// mutually-cyclic / self-cyclic nodes are invisible as a whole (check writes
// false first and then recurses, so every node on the cycle is caught by the
// continue at line 368), orphans (parent record missing) are promoted to
// roots, and an invisible parent hides its entire subtree — output is
// consistent and there is no panic. If 380-381 ever becomes reachable (e.g.
// check's semantics change to not recurse into the parent), this test's
// assertion combination will expose the behavioral divergence.
func TestCoverageFFilterAccessibleTreeDefensiveInvariants(t *testing.T) {
	parent1, parent2 := uint(1), uint(2)
	selfRing := uint(3)
	orphan := uint(99)
	hidden := uint(7)
	items := []model.MenuItem{
		{ID: 1, MenuKey: "ring-a", ParentID: &parent2, IsVisible: true},
		{ID: 2, MenuKey: "ring-b", ParentID: &parent1, IsVisible: true},
		{ID: 3, MenuKey: "self-ring", ParentID: &selfRing, IsVisible: true},
		{ID: 4, MenuKey: "orphan", ParentID: &orphan, IsVisible: true},
		{ID: 5, MenuKey: "root", IsVisible: true},
		{ID: 6, MenuKey: "under-hidden", ParentID: &hidden, IsVisible: true},
		{ID: 7, MenuKey: "hidden", IsVisible: false},
	}

	roots := filterAccessibleTree(items, nil)

	keys := make([]string, 0, len(roots))
	for _, root := range roots {
		keys = append(keys, root.MenuKey)
	}
	// Neither the A↔B mutual cycle, the self-ring, nor the hidden parent and
	// its children appear; the orphan is promoted to a root.
	// Order follows the items iteration order: orphan(4) before root(5).
	assert.Equal(t, []string{"orphan", "root"}, keys)
	for _, root := range roots {
		assert.Empty(t, root.Children, "%s 不应有任何子节点", root.MenuKey)
	}
}

// When Update is given all-blank labels, validateLabels reports
// "菜单名称不能为空" — covering the validateLabels failure branch inside Update
// (this branch lies outside the SetLabels-always-nil argument and is a
// reachable real validation path).
func TestCoverageFUpdateBlankLabelsRejected(t *testing.T) {
	service, ctx := newMenuTestService(t, "menu:create", "menu:update")

	created, err := service.Create(ctx, &CreateMenuRequest{MenuKey: "blank-labels", Labels: menuLabels()})
	require.NoError(t, err)

	blank := spec.LocalizedText{"zh-CN": "   "}
	_, err = service.Update(ctx, &UpdateMenuRequest{ID: formatID(created.ID), Labels: &blank})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "菜单名称不能为空")

	// Valid labels exercise the full validateLabels → SetLabels → landed-in-DB chain.
	rename := spec.LocalizedText{"zh-CN": "改名后的菜单", "en-US": "Renamed"}
	updated, err := service.Update(ctx, &UpdateMenuRequest{ID: formatID(created.ID), Labels: &rename})
	require.NoError(t, err)
	assert.Equal(t, map[string]string(rename), map[string]string(updated.Labels))
}
