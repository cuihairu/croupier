package function

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
)

func functionFormHistoryKey(functionID string) string {
	return "form." + strings.TrimSpace(functionID)
}

func snapshotFormCustomConfig(meta map[string]interface{}) map[string]interface{} {
	out := map[string]interface{}{}
	if meta == nil {
		return out
	}
	if v, ok := meta["form"]; ok {
		out["schema"] = v
	}
	return out
}

func applyFormCustomConfig(meta map[string]interface{}, cfg map[string]interface{}) map[string]interface{} {
	if meta == nil {
		meta = map[string]interface{}{}
	}
	if cfg == nil {
		cfg = map[string]interface{}{}
	}

	if schema, ok := cfg["schema"]; ok {
		meta["form"] = schema
	} else {
		delete(meta, "form")
	}
	delete(meta, "ui")
	delete(meta, "layout")
	delete(meta, "components")
	return meta
}

func persistFunctionFormVersion(ctx context.Context, svcCtx *svc.ServiceContext, fn *model.Function, message string) error {
	if svcCtx == nil || svcCtx.ConfigVersionModel == nil || fn == nil {
		return nil
	}
	snapshot := snapshotFormCustomConfig(fn.Metadata)
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
		Key:     functionFormHistoryKey(fn.FunctionID),
		Content: string(payload),
		Format:  "json",
		GameID:  gameID,
		Env:     env,
		Message: strings.TrimSpace(message),
	}, username)
	return err
}
