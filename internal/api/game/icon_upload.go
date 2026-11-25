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

// IconMaxBytes is the upload size limit for game icons.
//
// Icons render at 24-64px in lists/dropdowns (the GameIcon spec caps at
// 64x64), so 2MB is plenty for high-resolution SVG/PNG; anything larger is
// almost certainly mistakenly uploaded artwork rather than an icon.
const IconMaxBytes = 2 << 20 // 2MB

// GameIconUploadResponse is the icon upload result: key is the raw object
// storage key, and url is the currently directly accessible address (a
// relative /uploads/icons/... path when the file driver has no PublicURL
// configured, an absolute address when PublicURL is set). The frontend feeds
// url back into the game.icon form field.
type GameIconUploadResponse struct {
	Key string `json:"key"`
	URL string `json:"url"`
}

// openIconMultipartFileHeader is a test seam for multipart.FileHeader.Open
// (following the storage package precedent): in real requests gin has already
// opened the same FileHeader once, so a second Open failing on the handler
// side is unreachable and only tests can inject a fault to verify it.
var openIconMultipartFileHeader = func(fh *multipart.FileHeader) (multipart.File, error) {
	return fh.Open()
}

// UploadIcon handles game icon file upload (multipart field name file).
//
// Difference from the generic storage API: this endpoint produces **public
// icons directly loadable by <img>**, so it brings its own content-type/size
// allowlist and content-addressed naming; the generic /storage/objects writes
// private objects, and the two are not interchangeable.
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

// UploadIcon validates and writes the icon to object storage, returning the
// object key and an accessible URL.
//
// Overwrite policy (decided: content addressing): key =
// icons/games/<first 16 hex of sha1>.<ext>; re-uploading identical content
// yields the same key with the later write overwriting the earlier one
// (idempotent), and differing content always yields a different key, which
// rules out the "game A's logo.png overwrites game B's icon of the same
// name" collision problem at the root. The cost is that superseded icons
// become orphaned files (never cleaned up), at KB scale each, which is
// acceptable.
//
// Only the file storage driver is supported: an icon URL must be stable and
// directly loadable long term, while S3/OSS/COS SignedURLs carry an expiry,
// so storing one in game.icon is a dead link waiting to happen (the same
// lesson on the avatar side, see objstore/avatar.go). Object-storage
// deployments should fill in a CDN/external URL directly in the icon field.
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

	// The size is clamped to IconMaxBytes, so reading it fully into memory at
	// once is safe.
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

	// The file driver's SignedURL carries no signature (PublicURL or a
	// relative /uploads/ path), so it stays stable long term.
	url, err := s.svcCtx.ObjectStore.SignedURL(ctx, key, "GET", time.Hour)
	if err != nil {
		return nil, errorx.NewInternalError("生成图标访问地址失败: " + err.Error())
	}
	return &GameIconUploadResponse{Key: key, URL: url}, nil
}

// sniffIconKind determines the icon type from file-header magic numbers
// (neither the extension nor Content-Type is trusted).
//
// It returns the canonical extension and MIME type; anything unrecognized
// outside the allowlist is rejected outright -- icons are served directly
// from a publicly reachable static directory, so allowing arbitrary content
// would open an unauthenticated file upload surface.
func sniffIconKind(data []byte) (ext string, contentType string, err error) {
	switch {
	// PNG: 89 50 4E 47 0D 0A 1A 0A
	case len(data) >= 8 && data[0] == 0x89 && data[1] == 0x50 && data[2] == 0x4E && data[3] == 0x47 &&
		data[4] == 0x0D && data[5] == 0x0A && data[6] == 0x1A && data[7] == 0x0A:
		return "png", "image/png", nil
	// JPEG: FF D8 FF
	case len(data) >= 3 && data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF:
		return "jpg", "image/jpeg", nil
	// WEBP: RIFF....WEBP
	case len(data) >= 12 && string(data[0:4]) == "RIFF" && string(data[8:12]) == "WEBP":
		return "webp", "image/webp", nil
	}
	// SVG is a text format: detect the <svg element (the XML declaration is
	// optional), case-insensitively.
	head := data
	if len(head) > 512 {
		head = head[:512]
	}
	if strings.Contains(strings.ToLower(string(head)), "<svg") {
		return "svg", "image/svg+xml", nil
	}
	// Rejecting out-of-allowlist content means the client sent the wrong file
	// (400), not a server-side failure.
	return "", "", errorx.NewBadRequest("仅支持 png/jpg/webp/svg 格式的图标")
}
