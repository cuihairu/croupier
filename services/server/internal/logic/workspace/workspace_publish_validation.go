package workspace

import (
	"fmt"

	"github.com/cuihairu/croupier/services/server/internal/common/errorx"
	"github.com/cuihairu/croupier/services/server/internal/types"
)

var supportedPublishedTabLayoutTypes = map[string]struct{}{
	"form-detail": {},
	"list":        {},
	"form":        {},
	"detail":      {},
	"kanban":      {},
	"timeline":    {},
	"split":       {},
	"wizard":      {},
	"dashboard":   {},
	"grid":        {},
	"custom":      {},
}

func validateWorkspaceForPublish(cfg types.WorkspaceConfig) error {
	layout, ok := cfg.Layout.(map[string]interface{})
	if !ok || layout == nil {
		return errorx.NewBadRequest("layout must be an object")
	}

	layoutType, _ := layout["type"].(string)
	if layoutType != "tabs" {
		return errorx.NewBadRequest(fmt.Sprintf("top layout.type must be tabs, got %q", layoutType))
	}

	rawTabs, ok := layout["tabs"].([]interface{})
	if !ok {
		return errorx.NewBadRequest("tabs must be an array")
	}

	for i, rawTab := range rawTabs {
		tab, ok := rawTab.(map[string]interface{})
		if !ok {
			return errorx.NewBadRequest(fmt.Sprintf("tabs[%d] must be an object", i))
		}
		tabLayout, ok := tab["layout"].(map[string]interface{})
		if !ok || tabLayout == nil {
			return errorx.NewBadRequest(fmt.Sprintf("tabs[%d].layout must be an object", i))
		}
		tabLayoutType, _ := tabLayout["type"].(string)
		if _, supported := supportedPublishedTabLayoutTypes[tabLayoutType]; !supported {
			return errorx.NewBadRequest(
				fmt.Sprintf("tabs[%d].layout.type %q is not publishable, only form-detail/list/form/detail/kanban/timeline/split/wizard/dashboard/grid/custom", i, tabLayoutType),
			)
		}

		if tabLayoutType == "split" {
			panels, ok := tabLayout["panels"].([]interface{})
			if !ok || len(panels) == 0 {
				return errorx.NewBadRequest(fmt.Sprintf("tabs[%d].layout.panels must be a non-empty array", i))
			}
		}
		if tabLayoutType == "wizard" {
			steps, ok := tabLayout["steps"].([]interface{})
			if !ok || len(steps) == 0 {
				return errorx.NewBadRequest(fmt.Sprintf("tabs[%d].layout.steps must be a non-empty array", i))
			}
		}
		if tabLayoutType == "dashboard" {
			stats, statsOK := tabLayout["stats"].([]interface{})
			panels, panelsOK := tabLayout["panels"].([]interface{})
			if (!statsOK || len(stats) == 0) && (!panelsOK || len(panels) == 0) {
				return errorx.NewBadRequest(fmt.Sprintf("tabs[%d].layout dashboard requires stats or panels", i))
			}
		}
		if tabLayoutType == "grid" {
			items, ok := tabLayout["items"].([]interface{})
			if !ok || len(items) == 0 {
				return errorx.NewBadRequest(fmt.Sprintf("tabs[%d].layout.items must be a non-empty array", i))
			}
		}
		if tabLayoutType == "custom" {
			component, _ := tabLayout["component"].(string)
			if component == "" {
				return errorx.NewBadRequest(fmt.Sprintf("tabs[%d].layout.component is required for custom layout", i))
			}
		}
	}

	return nil
}
