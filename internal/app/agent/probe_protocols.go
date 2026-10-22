package agent

// 游戏协议探测适配器（probe.targets[].protocol 名 → core/healthprobe 层 3
// Checker）经 init 注册进 core/healthprobe 的适配器注册表；blank import 使
// 其随 agent 二进制生效（db driver 注册同款装配手法，见 probe_drivers.go）。
// core/healthprobe 保持零外部依赖，游戏协议知识只住各适配器包内。
import (
	_ "github.com/cuihairu/croupier/core/healthprobe/games/minecraft"
)
