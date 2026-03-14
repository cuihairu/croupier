package meta

import (
	"context"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/cuihairu/croupier/internal/svc"
)

type Service struct {
	svcCtx *svc.ServiceContext
}

func NewService(svcCtx *svc.ServiceContext) *Service {
	return &Service{svcCtx: svcCtx}
}

// Root returns API information and version
func (s *Service) Root(ctx context.Context) (*RootResponse, error) {
	profiles := make([]string, 0, len(s.svcCtx.Config.Profiles))
	for name := range s.svcCtx.Config.Profiles {
		profiles = append(profiles, name)
	}
	sort.Strings(profiles)

	data := map[string]interface{}{
		"service":     "croupier-server",
		"version":     currentAPIVersion(),
		"environment": s.svcCtx.Config.Server.Mode,
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

	return &RootResponse{
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
