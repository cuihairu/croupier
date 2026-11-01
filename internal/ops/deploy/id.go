package deploy

import (
	"crypto/rand"
	"encoding/hex"
	"time"
)

// NewDeploymentID 生成发版单 id（时间前缀 + 随机段，短而唯一）。
func NewDeploymentID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		// 熵源失败极罕见；回退时间戳保证仍唯一可排。
		return "dep-" + time.Now().UTC().Format("20060102150405.000000000")
	}
	return "dep-" + time.Now().UTC().Format("20060102150405") + "-" + hex.EncodeToString(b)
}

// NewPackageID 生成包 id。
func NewPackageID() string {
	return NewDeploymentID()
}
