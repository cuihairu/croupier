// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package meta

import (
	"context"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type RootLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 根路径 - API 信息和版本
func NewRootLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RootLogic {
	return &RootLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *RootLogic) Root(req *types.RootRequest) (resp *types.RootResponse, err error) {
	profiles := make([]string, 0, len(l.svcCtx.Config.Profiles))
	for name := range l.svcCtx.Config.Profiles {
		profiles = append(profiles, name)
	}
	sort.Strings(profiles)

	data := map[string]interface{}{
		"service":     "croupier-server",
		"version":     currentAPIVersion(),
		"environment": l.svcCtx.Config.RestConf.Mode,
		"timestamp":   utils.FormatTimestamp(time.Now()),
		"features": []string{
			"alerts",
			"analytics",
			"functions",
			"registry",
			"ops",
			"feedback",
		},
		"profiles": profiles,
		"links": map[string]string{
			"docs":   "https://github.com/cuihairu/croupier",
			"status": "/api/v1/ops/config",
			"health": "/api/v1/ops/health",
		},
	}

	return &types.RootResponse{
		Code:    0,
		Message: "OK",
		Data:    data,
	}, nil
}

var (
	versionOnce sync.Once
	apiVersion  string
)

func currentAPIVersion() string {
	versionOnce.Do(func() {
		if v := strings.TrimSpace(os.Getenv("CROUPIER_VERSION")); v != "" {
			apiVersion = v
			return
		}
		if v := readVersionFile(); v != "" {
			apiVersion = v
			return
		}
		apiVersion = "dev"
	})
	return apiVersion
}

func readVersionFile() string {
	data, err := os.ReadFile("VERSION")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}
