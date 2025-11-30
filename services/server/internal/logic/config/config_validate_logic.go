package config

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"strings"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/zeromicro/go-zero/core/logx"
	yaml "gopkg.in/yaml.v3"
)

type ConfigValidateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

type ConfigValidateRequest struct {
	Format  string `json:"format"`
	Content string `json:"content"`
}

func NewConfigValidateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ConfigValidateLogic {
	return &ConfigValidateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ConfigValidateLogic) ConfigValidate(req *ConfigValidateRequest) (map[string]interface{}, error) {
	if req == nil {
		return nil, errors.New("请求体不能为空")
	}
	format := strings.ToLower(strings.TrimSpace(req.Format))
	content := req.Content

	var err error
	switch format {
	case "json", "":
		var v interface{}
		err = json.Unmarshal([]byte(content), &v)
	case "yaml", "yml":
		var v interface{}
		err = yaml.Unmarshal([]byte(content), &v)
	case "xml":
		var v interface{}
		err = xml.Unmarshal([]byte(content), &v)
	default:
		// csv/ini/plain - accept as-is
		err = nil
	}

	if err != nil {
		return map[string]interface{}{
			"valid":  false,
			"errors": []string{err.Error()},
		}, nil
	}
	return map[string]interface{}{
		"valid": true,
	}, nil
}
