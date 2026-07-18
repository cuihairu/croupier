package function

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
)

func functionUIHistoryKey(functionID string) string {
	return "ui." + strings.TrimSpace(functionID)
}

func snapshotUICustomConfig(meta map[string]interface{}) map[string]interface{} {
	out := map[string]interface{}{}
	if meta == nil {
		return out
	}
	if v, ok := meta["ui"]; ok {
		out["schema"] = v
	}
	if v, ok := meta["layout"]; ok {
		out["layout"] = v
	}
	if v, ok := meta["components"]; ok {
		out["components"] = v
	}
	return out
}

func applyUICustomConfig(meta map[string]interface{}, cfg map[string]interface{}) map[string]interface{} {
	if meta == nil {
		meta = map[string]interface{}{}
	}
	if cfg == nil {
		cfg = map[string]interface{}{}
	}

	if schema, ok := cfg["schema"]; ok {
		meta["ui"] = schema
	} else {
		delete(meta, "ui")
	}
	if layout, ok := cfg["layout"]; ok {
		meta["layout"] = layout
	} else {
		delete(meta, "layout")
	}
	if components, ok := cfg["components"]; ok {
		meta["components"] = components
	} else {
		delete(meta, "components")
	}
	return meta
}

func persistFunctionUIVersion(ctx context.Context, svcCtx *svc.ServiceContext, fn *model.Function, message string) error {
	if svcCtx == nil || svcCtx.ConfigVersionModel == nil || fn == nil {
		return nil
	}
	snapshot := snapshotUICustomConfig(fn.Metadata)
	payload, err := json.Marshal(snapshot)
	if err != nil {
		return err
	}

	username, _ := utils.CurrentUsername(ctx)
	if strings.TrimSpace(username) == "" {
		username = "system"
	}

	// Extract game scope from context for multi-game isolation.
	gameID, env := svc.GameScopeFromContext(ctx)

	_, err = svcCtx.ConfigVersionModel.CreateWithMeta(ctx, model.ConfigVersionPayload{
		Key:     functionUIHistoryKey(fn.FunctionID),
		Content: string(payload),
		Format:  "json",
		GameID:  gameID,
		Env:     env,
		Message: strings.TrimSpace(message),
	}, username)
	return err
}
