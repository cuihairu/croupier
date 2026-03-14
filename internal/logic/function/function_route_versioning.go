package function

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
)

func functionRouteHistoryKey(functionID string) string {
	return "route." + strings.TrimSpace(functionID)
}

func persistFunctionRouteVersion(ctx context.Context, svcCtx *svc.ServiceContext, functionID string, route map[string]interface{}, message string) error {
	if svcCtx == nil || svcCtx.ConfigVersionModel == nil {
		return nil
	}
	payload, err := json.Marshal(route)
	if err != nil {
		return err
	}
	username, _ := utils.CurrentUsername(ctx)
	if strings.TrimSpace(username) == "" {
		username = "system"
	}
	_, err = svcCtx.ConfigVersionModel.CreateWithMeta(ctx, model.ConfigVersionPayload{
		Key:     functionRouteHistoryKey(functionID),
		Content: string(payload),
		Format:  "json",
		Message: strings.TrimSpace(message),
	}, username)
	return err
}
