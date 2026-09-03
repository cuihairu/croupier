// F：文件下发原语（hotpatch P1 传输层）——wire 编解码 + 校验 + 暂存落盘。
//
// 安全链全部强制：总开关 → 大小上限 → 仅 basename（拒穿越）→
// sha256 → 原子落盘暂存目录。**不自动应用**——应用由后续
// hotpatch runner 单独编排。
//
// wire（protobuf 兼容，与 Go/Java/JS/Python 手写编解码同构）：
//   FilePushRequest  { 1: transferId, 2: fileName, 3: contentSha256(hex), 4: data }
//   FilePushResponse { 1: transferId, 2: ok, 3: storedPath, 4: error }

#pragma once

#include "croupier/sdk/croupier_client.h"

#include <nlohmann/json.hpp>
#include <openssl/sha.h>

#include <cstdio>
#include <string>
#include <vector>

namespace croupier::sdk {

struct FilePushRequest {
    std::string transfer_id;
    std::string file_name;
    std::string content_sha256;
    std::vector<uint8_t> data;
};

struct FilePushResponse {
    std::string transfer_id;
    bool ok = false;
    std::string stored_path;
    std::string error;
};

inline void AppendVarint(std::vector<uint8_t>& out, uint64_t value) {
    while (value >= 0x80) {
        out.push_back(static_cast<uint8_t>(value) | 0x80);
        value >>= 7;
    }
    out.push_back(static_cast<uint8_t>(value));
}

inline std::string Sha256Hex(const std::vector<uint8_t>& data) {
    unsigned char digest[SHA256_DIGEST_LENGTH];
    SHA256(data.data(), data.size(), digest);
    static const char* hex = "0123456789abcdef";
    std::string out;
    out.reserve(SHA256_DIGEST_LENGTH * 2);
    for (unsigned char byte : digest) {
        out.push_back(hex[byte >> 4]);
        out.push_back(hex[byte & 0x0F]);
    }
    return out;
}

// 手写 protobuf wire 解码（length-delimited 四字段，未知字段跳过）。
inline FilePushRequest DecodeFilePushRequest(const std::vector<uint8_t>& body) {
    FilePushRequest req;
    size_t idx = 0;
    auto readVarint = [&](uint64_t& value) -> bool {
        value = 0;
        int shift = 0;
        while (idx < body.size()) {
            uint8_t byte = body[idx++];
            value |= static_cast<uint64_t>(byte & 0x7F) << shift;
            if (!(byte & 0x80)) return true;
            shift += 7;
            if (shift > 63) return false;
        }
        return false;
    };
    auto readBytes = [&](std::vector<uint8_t>& value) -> bool {
        uint64_t length = 0;
        if (!readVarint(length)) return false;
        if (idx + length > body.size()) return false;
        value.assign(body.begin() + static_cast<long>(idx),
                     body.begin() + static_cast<long>(idx + length));
        idx += length;
        return true;
    };
    while (idx < body.size()) {
        uint64_t tag = 0;
        if (!readVarint(tag)) return req;
        const uint64_t field = tag >> 3;
        std::vector<uint8_t> value;
        if (!readBytes(value)) return req;
        switch (field) {
            case 1:
                req.transfer_id.assign(value.begin(), value.end());
                break;
            case 2:
                req.file_name.assign(value.begin(), value.end());
                break;
            case 3:
                req.content_sha256.assign(value.begin(), value.end());
                break;
            case 4:
                req.data = value;
                break;
            default:
                break;  // 未知字段跳过
        }
    }
    return req;
}

inline std::vector<uint8_t> EncodeFilePushResponse(const FilePushResponse& resp) {
    auto fieldString = [](uint64_t field, const std::string& value) {
        std::vector<uint8_t> out;
        if (value.empty()) return out;
        AppendVarint(out, (field << 3) | 2);
        AppendVarint(out, value.size());
        out.insert(out.end(), value.begin(), value.end());
        return out;
    };
    std::vector<uint8_t> out;
    auto transfer = fieldString(1, resp.transfer_id);
    out.insert(out.end(), transfer.begin(), transfer.end());
    if (resp.ok) {
        out.push_back(0x10);  // field 2 varint
        out.push_back(0x01);
    }
    auto stored = fieldString(3, resp.stored_path);
    out.insert(out.end(), stored.begin(), stored.end());
    auto error = fieldString(4, resp.error);
    out.insert(out.end(), error.begin(), error.end());
    return out;
}

// 校验文件名仅含 basename 且落点仍在暂存目录内。
inline bool SafeStagingPath(const std::string& stagingDir, const std::string& fileName,
                            std::string& outPath) {
    if (fileName.empty() || fileName == "." || fileName == "..") return false;
    if (fileName.find('/') != std::string::npos ||
        fileName.find('\\') != std::string::npos ||
        fileName.find("..") != std::string::npos) {
        return false;
    }
    outPath = stagingDir + "/" + fileName;
    return true;
}

// 原子写：同目录临时文件 + rename。
inline bool AtomicWriteFile(const std::string& target, const std::vector<uint8_t>& data) {
    const std::string tmp = target + ".push-tmp";
    std::FILE* file = std::fopen(tmp.c_str(), "wb");
    if (!file) return false;
    if (!data.empty()) {
        if (std::fwrite(data.data(), 1, data.size(), file) != data.size()) {
            std::fclose(file);
            std::remove(tmp.c_str());
            return false;
        }
    }
    if (std::fclose(file) != 0) {
        std::remove(tmp.c_str());
        return false;
    }
    return std::rename(tmp.c_str(), target.c_str()) == 0;
}

}  // namespace croupier::sdk
