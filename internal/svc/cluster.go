package svc

import "github.com/cuihairu/croupier/internal/cluster"

// ClusterRuntime 暴露多实例 HA 的运行句柄（诊断/转发接线用）。
type ClusterRuntime struct {
	InstanceID string
	Epoch      uint64
	Mesh       *cluster.MeshInterconnect
}
