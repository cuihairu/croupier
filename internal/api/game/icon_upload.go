package game

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"io"
	"mime/multipart"
	"strings"
	"time"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/common/response"
	"github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/cuihairu/croupier/internal/platform/objstore"
	"github.com/gin-gonic/gin"
)

// IconMaxBytes 是游戏图标上传的大小上限。
//
// 图标在列表/下拉里按 24~64px 渲染（GameIcon 规范上限 64x64），2MB 足够容纳
// 高分辨率 SVG/PNG；再大的文件基本是误传素材而非图标。
const IconMaxBytes = 2 << 20 // 2MB

// GameIconUploadResponse 是图标上传的返回：key 为对象存储裸 key，url 为当前
// 可直接访问的地址（file 驱动未配置 PublicURL 时为 /uploads/icons/... 相对
// 路径，配置了 PublicURL 时为绝对地址）。前端把 url 回填进 game.icon 表单字段。
type GameIconUploadResponse struct {
	Key string `json:"key"`
	URL string `json:"url"`
}

// openIconMultipartFileHeader 是 multipart.FileHeader.Open 的测试缝隙（同
// storage 包先例）：真实请求中 gin 已对同一 FileHeader Open 过一次，handler
// 侧第二次 Open 失败不可达，仅测试可注入故障验证。
var openIconMultipartFileHeader = func(fh *multipart.FileHeader) (multipart.File, error) {
	return fh.Open()
}

// UploadIcon 处理游戏图标文件上传（multipart 字段名 file）。
//
// 与通用存储 API 的区别：本端点产出**可被 <img> 直接加载的公有图标**，因此
// 自带内容类型/大小白名单与内容寻址命名；通用 /storage/objects 写入的是私有
// 对象，两者不可互替。
func (h *Handler) UploadIcon(c *gin.Context) {
	fh, err := c.FormFile("file")
	if err != nil {
		response.Error(c, errorx.NewBadRequest("请通过 multipart 字段 file 提供图标文件"))
		return
	}
	file, err := openIconMultipartFileHeader(fh)
	if err != nil {
		response.Error(c, errorx.NewInternalError("读取上传文件失败: "+err.Error()))
		return
	}
	defer func() { _ = file.Close() }()

	resp, err := h.service.UploadIcon(c.Request.Context(), file, fh.Size, fh.Filename)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// UploadIcon 校验并把图标写入对象存储，返回对象 key 与可访问 URL。
//
// 覆盖策略（拍板：内容寻址）：key = icons/games/<sha1 前 16 位>.<扩展名>，
// 同内容重复上传得到同一 key、后写覆盖先写（幂等）；不同内容 key 必不同，
// 从根上排除「A 游戏的 logo.png 覆盖 B 游戏同名图标」的互踩问题。代价是旧
// 图标成为孤儿文件（不被清理），单枚以 KB 计、可接受。
//
// 仅支持 file 存储驱动：图标 URL 必须长期稳定可直载，S3/OSS/COS 驱动的
// SignedURL 带过期时间，存进 game.icon 就是定时失效的死链（头像侧同一教训
// 见 objstore/avatar.go）。对象存储部署请直接在图标字段填 CDN/外部 URL。
func (s *Service) UploadIcon(ctx context.Context, r io.Reader, size int64, _ string) (*GameIconUploadResponse, error) {
	if _, _, err := utils.RequireAnyPermission(ctx, s.svcCtx, "无权上传游戏图标", "admin:all", "games:write"); err != nil {
		return nil, err
	}
	if s.svcCtx == nil || s.svcCtx.ObjectStore == nil {
		return nil, errorx.NewInternalError("对象存储未初始化，无法上传图标")
	}
	if !strings.EqualFold(strings.TrimSpace(s.svcCtx.Config.Storage.Driver), "file") {
		return nil, errorx.NewBadRequest(
			"游戏图标上传当前仅支持本地存储（file）驱动；对象存储部署请直接填写图标 URL")
	}
	if size <= 0 {
		return nil, errorx.NewBadRequest("图标文件为空")
	}
	if size > IconMaxBytes {
		return nil, errorx.NewBadRequestWithDetails(
			"图标文件超过大小上限", map[string]any{"limit": IconMaxBytes, "actual": size})
	}

	// 大小已钳到 IconMaxBytes，一次性读入内存安全。
	data := make([]byte, size)
	if _, err := io.ReadFull(r, data); err != nil {
		return nil, errorx.NewBadRequest("读取图标内容失败: " + err.Error())
	}

	ext, contentType, err := sniffIconKind(data)
	if err != nil {
		return nil, err
	}

	sum := sha1.Sum(data)
	key := objstore.IconGamesPrefix + hex.EncodeToString(sum[:])[:16] + "." + ext
	if err := s.svcCtx.ObjectStore.Put(ctx, key, bytes.NewReader(data), int64(len(data)), contentType); err != nil {
		return nil, errorx.NewInternalError("保存图标失败: " + err.Error())
	}

	// file 驱动的 SignedURL 不带签名（PublicURL 或 /uploads/ 相对路径），长期稳定。
	url, err := s.svcCtx.ObjectStore.SignedURL(ctx, key, "GET", time.Hour)
	if err != nil {
		return nil, errorx.NewInternalError("生成图标访问地址失败: " + err.Error())
	}
	return &GameIconUploadResponse{Key: key, URL: url}, nil
}

// sniffIconKind 按文件头魔数判定图标类型（不信任扩展名与 Content-Type）。
//
// 返回规范的扩展名与 MIME；识别不出白名单类型一律拒绝——图标会被静态目录
// 公开直载，放行任意内容等于开放一个无鉴权文件上传面。
func sniffIconKind(data []byte) (ext string, contentType string, err error) {
	switch {
	// PNG：89 50 4E 47 0D 0A 1A 0A
	case len(data) >= 8 && data[0] == 0x89 && data[1] == 0x50 && data[2] == 0x4E && data[3] == 0x47 &&
		data[4] == 0x0D && data[5] == 0x0A && data[6] == 0x1A && data[7] == 0x0A:
		return "png", "image/png", nil
	// JPEG：FF D8 FF
	case len(data) >= 3 && data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF:
		return "jpg", "image/jpeg", nil
	// WEBP：RIFF....WEBP
	case len(data) >= 12 && string(data[0:4]) == "RIFF" && string(data[8:12]) == "WEBP":
		return "webp", "image/webp", nil
	}
	// SVG 是文本格式：按 <svg 元素判定（XML 声明可选），大小写不敏感。
	head := data
	if len(head) > 512 {
		head = head[:512]
	}
	if strings.Contains(strings.ToLower(string(head)), "<svg") {
		return "svg", "image/svg+xml", nil
	}
	// 拒绝白名单外内容是客户端送错文件（400），不是服务端故障。
	return "", "", errorx.NewBadRequest("仅支持 png/jpg/webp/svg 格式的图标")
}
