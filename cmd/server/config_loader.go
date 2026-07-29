package main

import (
	"fmt"
	"os"

	"github.com/cuihairu/croupier/internal/config"
	"gopkg.in/yaml.v3"
)

func loadConfigFile(path string) (config.Config, error) {
	var c config.Config
	if path == "" {
		return c, fmt.Errorf("必须指定配置文件 (-f/--config)")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return c, fmt.Errorf("读取配置文件失败: %w", err)
	}

	expanded := os.ExpandEnv(string(data))
	if err := yaml.Unmarshal([]byte(expanded), &c); err != nil {
		return c, fmt.Errorf("解析配置文件失败: %w", err)
	}

	return c, nil
}
