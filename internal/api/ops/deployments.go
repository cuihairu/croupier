package ops

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/common/response"
	"github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/ops/deploy"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// ---- DTO ----

type DeployPackageCreateRequest struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	GameID      string `json:"gameId"`
	Env         string `json:"env"`
	Role        string `json:"role"`
	ContentType string `json:"contentType"`
	// File 是 multipart 表单字段名（multipart/form-data 上传）。
	File string `json:"file" form:"file"`
}

type DeployPackageItem struct {
	PackageID string `json:"packageId"`
	Name      string `json:"name"`
	Version   string `json:"version"`
	GameID    string `json:"gameId"`
	Env       string `json:"env"`
	Role      string `json:"role"`
	ObjectKey string `json:"objectKey"`
	Size      int64  `json:"size"`
	Checksum  string `json:"checksum"`
	CreatedAt string `json:"createdAt"`
}

type DeployPackageCreateResponse struct {
	Package DeployPackageItem `json:"package"`
}

type DeployPackageListResponse struct {
	Items []DeployPackageItem `json:"items"`
	Total int                 `json:"total"`
}

type DeploymentCreateRequest struct {
	PackageID    string   `json:"packageId"`
	Name         string   `json:"name"`
	GameID       string   `json:"gameId"`
	Env          string   `json:"env"`
	Role         string   `json:"role"`
	ProcessName  string   `json:"processName"`
	BatchPercent int      `json:"batchPercent"`
	Targets      []string `json:"targets"`
}

type DeploymentTargetView struct {
	AgentID        string `json:"agentId"`
	Batch          int    `json:"batch"`
	State          string `json:"state"`
	Message        string `json:"message,omitempty"`
	OldFingerprint string `json:"oldFingerprint,omitempty"`
	NewFingerprint string `json:"newFingerprint,omitempty"`
	UpdatedAt      string `json:"updatedAt,omitempty"`
}

type DeploymentView struct {
	DeploymentID string                 `json:"deploymentId"`
	PackageID    string                 `json:"packageId"`
	Name         string                 `json:"name"`
	GameID       string                 `json:"gameId"`
	Env          string                 `json:"env"`
	Role         string                 `json:"role"`
	ProcessName  string                 `json:"processName"`
	BatchPercent int                    `json:"batchPercent"`
	BatchCount   int                    `json:"batchCount"`
	State        string                 `json:"state"`
	Targets      []DeploymentTargetView `json:"targets"`
	PausedReason string                 `json:"pausedReason,omitempty"`
	CreatedBy    string                 `json:"createdBy"`
	CreatedAt    string                 `json:"createdAt"`
	UpdatedAt    string                 `json:"updatedAt"`
}

type DeploymentCreateResponse struct {
	Deployment DeploymentView `json:"deployment"`
}

type DeploymentListResponse struct {
	Items []DeploymentView `json:"items"`
	Total int              `json:"total"`
}

type DeploymentActionResponse struct {
	Deployment DeploymentView `json:"deployment"`
}

// errDeployNotConfigured 部署编排未装配（DB/对象存储缺）：如实报 503，
// 不假装已开启。
var errDeployNotConfigured = &errorx.CodeError{Code: http.StatusServiceUnavailable, Message: "部署编排未配置", StableCode: "deploy_not_configured"}

// ---- handler ----

func (h *Handler) DeployPackageUpload(c *gin.Context) {
	// multipart：name/version/role 走表单字段，file 走文件字段。
	name := strings.TrimSpace(c.PostForm("name"))
	version := strings.TrimSpace(c.PostForm("version"))
	gameID := strings.TrimSpace(c.PostForm("gameId"))
	env := strings.TrimSpace(c.PostForm("env"))
	role := strings.TrimSpace(c.PostForm("role"))
	contentType := strings.TrimSpace(c.PostForm("contentType"))
	fileHeader, err := c.FormFile("file")
	if err != nil {
		response.BadRequest(c, "file 必填")
		return
	}
	f, err := fileHeader.Open()
	if err != nil {
		response.Error(c, errorx.NewInternalError("打开上传文件失败"))
		return
	}
	defer f.Close()
	// 上传包大小上限（500 MiB）：io.ReadAll 无上限可无界读入内存耗尽。
	const maxUploadSize = 500 << 20
	payload, err := io.ReadAll(io.LimitReader(f, maxUploadSize+1))
	if err != nil {
		response.Error(c, errorx.NewInternalError("读取上传内容失败"))
		return
	}
	if len(payload) > maxUploadSize {
		response.Error(c, errorx.NewBadRequest("上传包超过 500 MiB 上限"))
		return
	}
	req := &DeployPackageCreateRequest{
		Name: name, Version: version, GameID: gameID, Env: env,
		Role: role, ContentType: contentType, File: fileHeader.Filename,
	}
	resp, err := h.service.DeployPackageUpload(c.Request.Context(), req, payload)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) DeployPackageList(c *gin.Context) {
	resp, err := h.service.DeployPackageList(c.Request.Context())
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) DeploymentCreate(c *gin.Context) {
	var req DeploymentCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	resp, err := h.service.DeploymentCreate(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) DeploymentList(c *gin.Context) {
	resp, err := h.service.DeploymentList(c.Request.Context())
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) DeploymentGet(c *gin.Context) {
	resp, err := h.service.DeploymentGet(c.Request.Context(), c.Param("id"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) DeploymentStart(c *gin.Context) {
	resp, err := h.service.DeploymentStart(c.Request.Context(), c.Param("id"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) DeploymentRollback(c *gin.Context) {
	var req DeploymentRollbackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	resp, err := h.service.DeploymentRollback(c.Request.Context(), c.Param("id"), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// DeploymentRollbackRequest 回滚：指回上一版本包（上一版本需已登记）。
type DeploymentRollbackRequest struct {
	RollbackPackageID string `json:"rollbackPackageId"`
}

// ---- service ----

func (s *Service) DeployPackageUpload(ctx context.Context, req *DeployPackageCreateRequest, payload []byte) (*DeployPackageCreateResponse, error) {
	if s.svcCtx == nil || s.svcCtx.ObjectStore == nil {
		return nil, errDeployNotConfigured
	}
	name := strings.TrimSpace(req.Name)
	version := strings.TrimSpace(req.Version)
	if name == "" || version == "" {
		return nil, errorx.NewBadRequest("name 与 version 必填")
	}
	role := strings.TrimSpace(req.Role)
	if role == "" {
		role = "agent"
	}
	// role/name/version 拼接对象键与 agent 暂存路径，须拒绝路径分隔符与
	// 遍历序列（防任意对象键写 / 任意路径写）。
	if n, err := safeObjectPathPart(name); err != nil {
		return nil, errorx.NewBadRequest("name: " + err.Error())
	} else {
		name = n
	}
	if v, err := safeObjectPathPart(version); err != nil {
		return nil, errorx.NewBadRequest("version: " + err.Error())
	} else {
		version = v
	}
	if r, err := safeObjectPathPart(role); err != nil {
		return nil, errorx.NewBadRequest("role: " + err.Error())
	} else {
		role = r
	}
	contentType := strings.TrimSpace(req.ContentType)
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	if len(payload) == 0 {
		return nil, errorx.NewBadRequest("file 内容为空")
	}
	pid := newPackageID()
	// 审核发现 726：对象键包含唯一 packageID，同 (role,name,version) 重复
	// 上传不再覆盖旧对象。历史 DeployPackage 记录的 Checksum 与 ObjectKey
	// 指向的对象内容保持一致，供应链完整性不被后续上传破坏。
	key := fmt.Sprintf("deploy/%s/%s-%s-%s.bin", role, name, version, pid)
	hasher := sha256.New()
	hasher.Write(payload)
	checksum := hex.EncodeToString(hasher.Sum(nil))
	size := int64(len(payload))
	reader := newBytesReader(payload)
	if err := s.svcCtx.ObjectStore.Put(ctx, key, reader, size, contentType); err != nil {
		return nil, errorx.NewInternalError("上传对象存储失败: " + err.Error())
	}

	pkg := &model.DeployPackage{
		PackageID:   pid,
		Name:        name,
		Version:     version,
		GameID:      strings.TrimSpace(req.GameID),
		Env:         strings.TrimSpace(req.Env),
		Role:        role,
		ObjectKey:   key,
		Size:        size,
		Checksum:    checksum,
		ContentType: contentType,
		CreatedBy:   currentUsername(ctx),
	}
	if err := s.svcCtx.DeployPackageModel.Create(ctx, pkg); err != nil {
		return nil, errorx.NewInternalError("登记包失败: " + err.Error())
	}
	return &DeployPackageCreateResponse{Package: buildDeployPackageView(pkg)}, nil
}

func (s *Service) DeployPackageList(ctx context.Context) (*DeployPackageListResponse, error) {
	if s.svcCtx == nil || s.svcCtx.DeployPackageModel == nil {
		return nil, errDeployNotConfigured
	}
	items, err := s.svcCtx.DeployPackageModel.List(ctx, 100)
	if err != nil {
		return nil, errorx.NewInternalError("查询包失败")
	}
	out := make([]DeployPackageItem, 0, len(items))
	for i := range items {
		out = append(out, buildDeployPackageView(&items[i]))
	}
	return &DeployPackageListResponse{Items: out, Total: len(out)}, nil
}

func (s *Service) DeploymentCreate(ctx context.Context, req *DeploymentCreateRequest) (*DeploymentCreateResponse, error) {
	if s.svcCtx == nil || s.svcCtx.Deployments == nil {
		return nil, errDeployNotConfigured
	}
	if req.BatchPercent == 0 {
		req.BatchPercent = 100
	}
	d, err := deploy.NewDeployment(
		strings.TrimSpace(req.PackageID), strings.TrimSpace(req.Name),
		strings.TrimSpace(req.GameID), strings.TrimSpace(req.Env),
		strings.TrimSpace(req.Role), strings.TrimSpace(req.ProcessName),
		req.BatchPercent, req.Targets, currentUsername(ctx),
	)
	if err != nil {
		return nil, errorx.NewBadRequest(err.Error())
	}
	if err := s.svcCtx.DeploymentModel.Create(ctx, d); err != nil {
		return nil, errorx.NewInternalError("创建发版单失败: " + err.Error())
	}
	return &DeploymentCreateResponse{Deployment: buildDeploymentView(d)}, nil
}

func (s *Service) DeploymentList(ctx context.Context) (*DeploymentListResponse, error) {
	if s.svcCtx == nil || s.svcCtx.DeploymentModel == nil {
		return nil, errDeployNotConfigured
	}
	items, err := s.svcCtx.DeploymentModel.List(ctx, 100)
	if err != nil {
		return nil, errorx.NewInternalError("查询发版单失败")
	}
	out := make([]DeploymentView, 0, len(items))
	for i := range items {
		out = append(out, buildDeploymentView(&items[i]))
	}
	return &DeploymentListResponse{Items: out, Total: len(out)}, nil
}

func (s *Service) DeploymentGet(ctx context.Context, id string) (*DeploymentActionResponse, error) {
	if s.svcCtx == nil || s.svcCtx.DeploymentModel == nil {
		return nil, errDeployNotConfigured
	}
	d, err := s.svcCtx.DeploymentModel.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errorx.NewNotFound("发版单不存在")
		}
		return nil, errorx.NewInternalError("查询发版单失败")
	}
	return &DeploymentActionResponse{Deployment: buildDeploymentView(d)}, nil
}

func (s *Service) DeploymentStart(ctx context.Context, id string) (*DeploymentActionResponse, error) {
	if s.svcCtx == nil || s.svcCtx.Deployments == nil {
		return nil, errDeployNotConfigured
	}
	d, err := s.svcCtx.DeploymentModel.FindByID(ctx, id)
	if err != nil {
		return nil, errorx.NewNotFound("发版单不存在")
	}
	pkg, err := s.svcCtx.DeployPackageModel.FindByID(ctx, d.PackageID)
	if err != nil {
		return nil, errorx.NewNotFound("包不存在")
	}
	// 发版全链（分批推进 + 健康门禁）可持续数分钟，挂在 HTTP 请求 ctx 上会
	// 在客户端断开时取消整个发版（#666）；WithoutCancel 从入参派生后台
	// ctx（保留 game DB 路由等值，断开不取消）完成发版并落终态（CR-FIX-22）。
	bg, cancel := context.WithCancel(context.WithoutCancel(ctx))
	defer cancel()
	started, err := s.svcCtx.Deployments.Start(bg, d, pkg)
	if err != nil {
		return &DeploymentActionResponse{Deployment: buildDeploymentView(started)}, err
	}
	return &DeploymentActionResponse{Deployment: buildDeploymentView(started)}, nil
}

func (s *Service) DeploymentRollback(ctx context.Context, id string, req *DeploymentRollbackRequest) (*DeploymentActionResponse, error) {
	if s.svcCtx == nil || s.svcCtx.Deployments == nil {
		return nil, errDeployNotConfigured
	}
	d, err := s.svcCtx.DeploymentModel.FindByID(ctx, id)
	if err != nil {
		return nil, errorx.NewNotFound("发版单不存在")
	}
	if strings.TrimSpace(req.RollbackPackageID) == "" {
		return nil, errorx.NewBadRequest("rollbackPackageId 必填")
	}
	rbPkg, err := s.svcCtx.DeployPackageModel.FindByID(ctx, strings.TrimSpace(req.RollbackPackageID))
	if err != nil {
		return nil, errorx.NewNotFound("回滚包不存在")
	}
	// 回滚同发版语义：WithoutCancel 派生，断开不取消（CR-FIX-22）。
	bg, cancel := context.WithCancel(context.WithoutCancel(ctx))
	defer cancel()
	rolled, err := s.svcCtx.Deployments.Rollback(bg, d, rbPkg)
	if err != nil {
		return nil, err
	}
	return &DeploymentActionResponse{Deployment: buildDeploymentView(rolled)}, nil
}

// ---- helpers ----

func buildDeployPackageView(p *model.DeployPackage) DeployPackageItem {
	return DeployPackageItem{
		PackageID: p.PackageID,
		Name:      p.Name,
		Version:   p.Version,
		GameID:    p.GameID,
		Env:       p.Env,
		Role:      p.Role,
		ObjectKey: p.ObjectKey,
		Size:      p.Size,
		Checksum:  p.Checksum,
		CreatedAt: p.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
	}
}

func buildDeploymentView(d *model.Deployment) DeploymentView {
	targets := make([]DeploymentTargetView, 0)
	_ = json.Unmarshal(d.TargetsJSON, &targets)
	if targets == nil {
		targets = []DeploymentTargetView{}
	}
	return DeploymentView{
		DeploymentID: d.DeploymentID,
		PackageID:    d.PackageID,
		Name:         d.Name,
		GameID:       d.GameID,
		Env:          d.Env,
		Role:         d.Role,
		ProcessName:  d.ProcessName,
		BatchPercent: d.BatchPercent,
		BatchCount:   d.BatchCount,
		State:        d.State,
		Targets:      targets,
		PausedReason: d.PausedReason,
		CreatedBy:    d.CreatedBy,
		CreatedAt:    d.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		UpdatedAt:    d.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z"),
	}
}

// bytesReader 是 io.ReadSeeker 的 bytes.Reader 持久游标封装。
// 旧实现每次 Read/Seek 都新建 bytes.Reader（不保持游标），Read 永不返回
// EOF——objstore 各驱动 Put（io.Copy 直到 EOF）会死循环写无限增长。改为
// 持有单一 reader，Read/Seek 推进同一游标。
type bytesReader struct {
	mu     sync.Mutex
	reader *bytes.Reader
}

func newBytesReader(data []byte) *bytesReader {
	return &bytesReader{reader: bytes.NewReader(data)}
}

func (b *bytesReader) Read(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.reader.Read(p)
}

func (b *bytesReader) Seek(off int64, whence int) (int64, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.reader.Seek(off, whence)
}

// currentUsername 取认证上下文用户名（与既有 helpers 一致；此处独立兜底）。
func currentUsername(ctx context.Context) string {
	if name, err := utils.CurrentUsername(ctx); err == nil {
		return name
	}
	return "unknown"
}

// newPackageID 生成包 id。
func newPackageID() string {
	return deploy.NewPackageID()
}
