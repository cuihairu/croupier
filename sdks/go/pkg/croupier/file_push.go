package croupier

// F：Provider 侧文件下发原语（hotpatch P1 传输层切片 1）。
//
// 安全约束（全部强制，缺一拒绝）：
//   - 总开关 EnableFileTransfer（默认关）
//   - 大小上限 MaxFileSize（默认 10MB）
//   - 文件名仅允许 basename（拒绝路径穿越/绝对路径/隐藏分隔符）
//   - sha256 校验（传输完整性）
//   - 只落盘到暂存目录（FileStagingDir），**不自动应用**——应用由后续
//     hotpatch runner（备份→替换→自检→回滚）单独编排。
//
// wire（protobuf 兼容，手写编解码避免全 SDK 再生成）：
//   FilePushRequest  { 1: transfer_id, 2: file_name, 3: content_sha256(hex), 4: data }
//   FilePushResponse { 1: transfer_id, 2: ok, 3: stored_path, 4: error }

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// filePushRequest 手写解码结果。
type filePushRequest struct {
	transferID    string
	fileName      string
	contentSha256 string
	data          []byte
}

// encodeFilePushRequest 手写 protobuf 兼容编码（测试用）。
func encodeFilePushRequest(req *filePushRequest) []byte {
	var out []byte
	appendStringField := func(field int, value string) {
		if value == "" {
			return
		}
		out = append(out, byte(field<<3|2))
		out = appendVarint(out, uint64(len(value)))
		out = append(out, value...)
	}
	appendBytesField := func(field int, value []byte) {
		if len(value) == 0 {
			return
		}
		out = append(out, byte(field<<3|2))
		out = appendVarint(out, uint64(len(value)))
		out = append(out, value...)
	}
	appendStringField(1, req.transferID)
	appendStringField(2, req.fileName)
	appendStringField(3, req.contentSha256)
	appendBytesField(4, req.data)
	return out
}

func appendVarint(out []byte, value uint64) []byte {
	for value >= 0x80 {
		out = append(out, byte(value)|0x80)
		value >>= 7
	}
	return append(out, byte(value))
}

// readVarint 从 buf 偏移 idx 读取 varint，返回 (value, 新偏移)。
func readVarint(buf []byte, idx int) (uint64, int, error) {
	var value uint64
	var shift uint
	for {
		if idx >= len(buf) {
			return 0, idx, errors.New("truncated varint")
		}
		b := buf[idx]
		idx++
		value |= uint64(b&0x7F) << shift
		if b < 0x80 {
			return value, idx, nil
		}
		shift += 7
		if shift > 63 {
			return 0, idx, errors.New("varint overflow")
		}
	}
}

// decodeFilePushRequest 手写 protobuf 兼容解码（未知字段跳过）。
func decodeFilePushRequest(body []byte) (*filePushRequest, error) {
	req := &filePushRequest{}
	idx := 0
	for idx < len(body) {
		tag, next, err := readVarint(body, idx)
		if err != nil {
			return nil, err
		}
		idx = next
		field := int(tag >> 3)
		wireType := int(tag & 0x7)
		if wireType != 2 { // length-delimited
			return nil, fmt.Errorf("unsupported wire type %d", wireType)
		}
		length, next, err := readVarint(body, idx)
		if err != nil {
			return nil, err
		}
		idx = next
		if idx+int(length) > len(body) {
			return nil, errors.New("truncated field")
		}
		value := body[idx : idx+int(length)]
		idx += int(length)
		switch field {
		case 1:
			req.transferID = string(value)
		case 2:
			req.fileName = string(value)
		case 3:
			req.contentSha256 = string(value)
		case 4:
			req.data = append([]byte(nil), value...)
		}
	}
	return req, nil
}

// safeStagingPath 校验文件名仅含 basename 且落点仍在暂存目录内
// （防御路径穿越：../、绝对路径、分隔符变体）。
func safeStagingPath(stagingDir, fileName string) (string, error) {
	if strings.TrimSpace(fileName) == "" {
		return "", errors.New("file name is required")
	}
	if strings.ContainsAny(fileName, "/\\") || strings.Contains(fileName, "..") ||
		filepath.IsAbs(fileName) || fileName == "." || fileName == ".." {
		return "", fmt.Errorf("file name must be a bare basename: %q", fileName)
	}
	clean := filepath.Clean(filepath.Join(stagingDir, fileName))
	if !strings.HasPrefix(clean, filepath.Clean(stagingDir)+string(os.PathSeparator)) {
		return "", fmt.Errorf("resolved path escapes staging dir: %q", clean)
	}
	return clean, nil
}

// validateFilePush 对入站文件推送做全部安全与完整性校验。
// 返回错误消息（已含原因）；通过返回空串。
func (m *TCPManager) validateFilePush(req *filePushRequest) error {
	if !m.config.EnableFileTransfer {
		return errors.New("file transfer is disabled on this provider")
	}
	if strings.TrimSpace(req.transferID) == "" {
		return errors.New("transfer_id is required")
	}
	if _, err := safeStagingPath(m.fileStagingDir(), req.fileName); err != nil {
		return err
	}
	maxSize := m.config.MaxFileSize
	if maxSize <= 0 {
		maxSize = 10 * 1024 * 1024
	}
	if len(req.data) == 0 {
		return errors.New("file payload is empty")
	}
	if len(req.data) > maxSize {
		return fmt.Errorf("file size %d exceeds max %d", len(req.data), maxSize)
	}
	if strings.TrimSpace(req.contentSha256) == "" {
		return errors.New("content_sha256 is required")
	}
	sum := sha256.Sum256(req.data)
	if !strings.EqualFold(hex.EncodeToString(sum[:]), strings.TrimSpace(req.contentSha256)) {
		return errors.New("checksum mismatch")
	}
	return nil
}

// fileStagingDir 返回暂存目录（默认 ./croupier-staging），确保存在。
func (m *TCPManager) fileStagingDir() string {
	dir := strings.TrimSpace(m.config.FileStagingDir)
	if dir == "" {
		dir = "./croupier-staging"
	}
	_ = os.MkdirAll(dir, 0o750)
	return dir
}

// handleFilePushRequest 处理 agent → provider 的文件下发帧：
// 校验通过后原子落盘（tmp+rename）到暂存目录并回确认；失败回错误说明。
func (m *TCPManager) handleFilePushRequest(body []byte) ([]byte, error) {
	req, err := decodeFilePushRequest(body)
	if err != nil {
		return nil, fmt.Errorf("unmarshal FilePushRequest: %w", err)
	}

	var transferID, storedPath, message string
	ok := false
	if validateErr := m.validateFilePush(req); validateErr != nil {
		message = validateErr.Error()
	} else {
		stagingDir := m.fileStagingDir()
		target, pathErr := safeStagingPath(stagingDir, req.fileName)
		if pathErr != nil {
			message = pathErr.Error()
		} else if writeErr := atomicWriteFile(target, req.data); writeErr != nil {
			message = fmt.Sprintf("write staging file: %v", writeErr)
		} else {
			transferID = req.transferID
			ok = true
			storedPath = target
		}
	}
	return encodeFilePushResponse(filePushResponse{
		transferID: transferID,
		ok:         ok,
		storedPath: storedPath,
		error:      message,
	}), nil
}

// filePushResponse 手写编码结果。
type filePushResponse struct {
	transferID string
	ok         bool
	storedPath string
	error      string
}

// encodeFilePushResponse 手写 protobuf 兼容编码：
// { 1: transfer_id(string), 2: ok(bool), 3: stored_path(string), 4: error(string) }。
func encodeFilePushResponse(resp filePushResponse) []byte {
	var out []byte
	if resp.transferID != "" {
		out = append(out, 0x0A)
		out = appendVarint(out, uint64(len(resp.transferID)))
		out = append(out, resp.transferID...)
	}
	if resp.ok {
		out = append(out, 0x10, 0x01)
	}
	if resp.storedPath != "" {
		out = append(out, 0x1A)
		out = appendVarint(out, uint64(len(resp.storedPath)))
		out = append(out, resp.storedPath...)
	}
	if resp.error != "" {
		out = append(out, 0x22)
		out = appendVarint(out, uint64(len(resp.error)))
		out = append(out, resp.error...)
	}
	return out
}

// atomicWriteFile 先写同目录临时文件再 rename，避免半写文件被下游读到。
func atomicWriteFile(target string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(target), ".push-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	if err := os.Rename(tmpName, target); err != nil {
		os.Remove(tmpName)
		return err
	}
	return nil
}
