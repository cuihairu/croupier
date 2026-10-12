// Package vault 密码箱抽象（密码箱批 2026-10-13，todo 4249）：渠道与支付
// 商户密钥、上游服务与数据库凭据等敏感配置的统一存取面。persona 新升级
// 已实现密码箱、可作网站密码箱使用；各项目密码箱应抽象接口、支持替换
// 提供商——本包即该抽象，Provider 实现经编译期注册表装配（serverstatus/
// outlet 同构），切换提供商=只改配置 provider 键。
//
// 红线（todo 4250 批头）：密钥不进仓库；解锁口令与明文凭据不落日志、
// 不出现在错误消息。调用方取到的明文只允许内存传递，禁止写入日志/审计
// 载荷/错误包装。
package vault

import (
	"context"
	"errors"
	"strings"
	"time"
)

// 密码箱错误闭集（机器可分支；消息不含任何口令/明文凭据）。
var (
	// ErrLocked 会话未解锁或已超时（local 滑动 TTL；调用方重新 Unlock）。
	ErrLocked = errors.New("vault locked")
	// ErrNotFound 指定路径无凭据。
	ErrNotFound = errors.New("vault entry not found")
	// ErrPassphraseMissing 主口令环境变量未设置（口令不落配置文件）。
	ErrPassphraseMissing = errors.New("vault passphrase env not set")
	// ErrPassphraseMismatch 主口令不匹配或存储损坏（校验块解不开）。
	ErrPassphraseMismatch = errors.New("vault unlock failed: passphrase mismatch or store corrupted")
	// ErrReadOnly 提供商只读（如 env 源）。
	ErrReadOnly = errors.New("vault provider is read-only")
	// ErrInvalidPath 凭据路径非法（空、含 ".." 或段为空）。
	ErrInvalidPath = errors.New("vault entry path invalid")
)

// EntryMeta 凭据条目元数据（只含路径与更新时间，永不含明文）。
type EntryMeta struct {
	Path      string
	UpdatedAt time.Time
}

// Provider 密码箱抽象（persona 密码箱形态同构：解锁/读/写/列/删除）。
// 实现侧持会话语义（local：主口令 KDF + 滑动 TTL；env：恒解锁）。
type Provider interface {
	// Name 注册表键（ProviderLocal / ProviderEnv / ...）。
	Name() string
	// Unlock 解锁会话。local 从主口令环境变量取口令做 KDF 校验；重复
	// 调用=续期。失败返回 ErrPassphraseMissing / ErrPassphraseMismatch。
	Unlock(ctx context.Context) error
	// Locked 报告当前是否处于锁定态（Read/Write/List/Delete 前置）。
	Locked() bool
	// Lock 立即锁定（清零会话密钥；不删除存储）。
	Lock()
	// Read 取凭据明文（仅内存传递；未解锁 ErrLocked，无条目 ErrNotFound）。
	Read(ctx context.Context, path string) (string, error)
	// Write 写入/覆盖凭据（未解锁 ErrLocked；只读实现 ErrReadOnly）。
	Write(ctx context.Context, path, value string) error
	// List 列凭据路径与更新时间（不含明文；未解锁 ErrLocked）。
	List(ctx context.Context) ([]EntryMeta, error)
	// Delete 删除凭据（未解锁 ErrLocked；无条目 ErrNotFound；只读实现
	// ErrReadOnly）。
	Delete(ctx context.Context, path string) error
}

// ValidatePath 路径合法性（实现共用）：非空、无首尾空白、不含 ".."、
// 按 "/" 切分后无空段。凭据路径形如 "smtp/password"、"channels/dingtalk"。
func ValidatePath(path string) error {
	if path == "" || strings.TrimSpace(path) != path || strings.Contains(path, "..") {
		return ErrInvalidPath
	}
	for _, seg := range strings.Split(path, "/") {
		if seg == "" {
			return ErrInvalidPath
		}
	}
	return nil
}
