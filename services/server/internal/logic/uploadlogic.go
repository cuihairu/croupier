package logic

import (
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cuihairu/croupier/services/api/internal/svc"
	"github.com/cuihairu/croupier/services/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type UploadLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUploadLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UploadLogic {
	return &UploadLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UploadLogic) Upload(user string, file multipart.File, header *multipart.FileHeader) (*types.UploadResponse, error) {
	store := l.svcCtx.ObjStore()
	if store == nil {
		return nil, fmt.Errorf("storage not configured")
	}
	defer file.Close()
	tmp, err := os.CreateTemp("", "upload-*")
	if err != nil {
		return nil, err
	}
	defer func() {
		tmp.Close()
		os.Remove(tmp.Name())
	}()
	size, err := io.Copy(tmp, file)
	if err != nil {
		return nil, err
	}
	if _, err := tmp.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	key := buildObjectKey(user, header.Filename)
	contentType := header.Header.Get("Content-Type")
	if err := store.Put(l.ctx, key, tmp, size, contentType); err != nil {
		return nil, err
	}
	url := ""
	conf := l.svcCtx.ObjConfig()
	if conf.SignedURLTTL > 0 {
		if signed, err := store.SignedURL(l.ctx, key, "GET", conf.SignedURLTTL); err == nil {
			url = signed
		}
	}
	return &types.UploadResponse{
		Key: key,
		Url: url,
	}, nil
}

func buildObjectKey(user, filename string) string {
	u := sanitizeSegment(user)
	if u == "" {
		u = "user"
	}
	name := sanitizeFilename(filename)
	if name == "" {
		name = fmt.Sprintf("file_%d", time.Now().UnixNano())
	}
	return fmt.Sprintf("%s/%d_%s", u, time.Now().UnixNano(), name)
}

func sanitizeSegment(value string) string {
	value = strings.TrimSpace(value)
	buf := strings.Builder{}
	for _, r := range value {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-' || r == '_' {
			buf.WriteRune(r)
		} else if r >= 'A' && r <= 'Z' {
			buf.WriteRune(r + 32)
		}
	}
	return buf.String()
}

func sanitizeFilename(name string) string {
	if name == "" {
		return ""
	}
	base := filepath.Base(name)
	base = strings.ReplaceAll(base, "..", "")
	base = strings.ReplaceAll(base, string(os.PathSeparator), "_")
	base = strings.ReplaceAll(base, "/", "_")
	if base == "" {
		return ""
	}
	return base
}
