// 覆盖目标：Service.recordLoginAudit 的 ipRegion 分支——req.ClientIP 非空且
// ipgeo.Region 命中时，把归属地写入 metadata["ipRegion"]（service.go:580）。
//
// ipgeo 按 sync.Once 惰性初始化，环境变量只在进程内第一次 Region 调用时
// 读取一次。本文件名以 audit_ 开头，按文件名排序先于包内其余测试文件
// 编译执行，保证这里的合成 IP2Location BIN 是 once 初始化时唯一可见的
// 数据库。其余测试使用的 IP（127.0.0.1/10.0.0.1/192.168.1.1）落在哨兵
// 记录上，Region 仍返回空串，既有断言不受影响。
package auth

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	permissionservice "github.com/cuihairu/croupier/internal/service/permission"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ipRegionWriteString 追加长度前缀字符串（IP2Location 磁盘格式），返回其
// 在 buf 中的绝对偏移。
func ipRegionWriteString(buf *bytes.Buffer, s string) uint32 {
	off := uint32(buf.Len())
	buf.WriteByte(byte(len(s)))
	buf.WriteString(s)
	return off
}

// writeIPRegionSyntheticBIN 构造最小可用的 IP2Location DB3 格式 IPv4 BIN
// （与 internal/ipgeo 的合成 BIN 同构）：
//
//	1.2.3.4         -> US / United States / California / Mountain View
//	2.2.2.2         -> CN / China
//	3.3.3.3         -> JP（短名回退）
//	255.255.255.255 -> 哨兵上界，其余地址查询返回空
func writeIPRegionSyntheticBIN(t *testing.T) string {
	t.Helper()
	buf := &bytes.Buffer{}

	hdr := make([]byte, 64)
	hdr[0] = 3  // databasetype: DB3（country/region/city）
	hdr[1] = 4  // databasecolumn
	hdr[2] = 20 // year 2020：绕过 productcode 校验
	hdr[3], hdr[4] = 1, 1
	binary.LittleEndian.PutUint32(hdr[5:], 3)  // ipv4 record count
	binary.LittleEndian.PutUint32(hdr[9:], 65) // ipv4 base addr（1-based 65 -> 偏移 64）
	buf.Write(hdr)

	// 4 个记录槽（每个 16 字节）：3 条数据 + 1 条哨兵；字符串区紧随其后。
	const strBase = 64 + 4*16
	strs := &bytes.Buffer{}
	cUS := strBase + uint32(strs.Len())
	ipRegionWriteString(strs, "US")
	// 长 country 名必须紧跟短名：库在 country_ptr+3 处读取（+3 跳过短名）。
	ipRegionWriteString(strs, "United States")
	rUS := strBase + uint32(strs.Len())
	ipRegionWriteString(strs, "California")
	cyUS := strBase + uint32(strs.Len())
	ipRegionWriteString(strs, "Mountain View")
	cCN := strBase + uint32(strs.Len())
	ipRegionWriteString(strs, "CN")
	ipRegionWriteString(strs, "China")
	rEmpty := strBase + uint32(strs.Len())
	ipRegionWriteString(strs, "")
	cyEmpty := strBase + uint32(strs.Len())
	ipRegionWriteString(strs, "")
	cJP := strBase + uint32(strs.Len())
	ipRegionWriteString(strs, "JP")
	ipRegionWriteString(strs, "")

	writeRec := func(from uint32, c, r, cy uint32) {
		var rec [16]byte
		binary.LittleEndian.PutUint32(rec[0:], from)
		binary.LittleEndian.PutUint32(rec[4:], c)
		binary.LittleEndian.PutUint32(rec[8:], r)
		binary.LittleEndian.PutUint32(rec[12:], cy)
		buf.Write(rec[:])
	}
	writeRec(0x01020304, cUS, rUS, cyUS)       // 1.2.3.4
	writeRec(0x02020202, cCN, rEmpty, cyEmpty) // 2.2.2.2
	writeRec(0x03030303, cJP, rEmpty, cyEmpty) // 3.3.3.3
	writeRec(0xFFFFFFFF, 0, 0, 0)              // 哨兵上界

	buf.Write(strs.Bytes())

	path := filepath.Join(t.TempDir(), "IP2LOCATION-LITE-DB3.BIN")
	require.NoError(t, os.WriteFile(path, buf.Bytes(), 0o600))
	return path
}

func TestRecordLoginAudit_IPRegionFromSyntheticDB(t *testing.T) {
	prev, hadPrev := os.LookupEnv("IP2LOCATION_BIN_PATH")
	require.NoError(t, os.Setenv("IP2LOCATION_BIN_PATH", writeIPRegionSyntheticBIN(t)))
	t.Cleanup(func() {
		if hadPrev {
			_ = os.Setenv("IP2LOCATION_BIN_PATH", prev)
		} else {
			_ = os.Unsetenv("IP2LOCATION_BIN_PATH")
		}
	})

	db := setupTestDB(t)
	svc := withTableAudit(t, db, NewService(
		model.NewAdminModel(db),
		permissionservice.NewPermissionService(db),
		"secret",
	))

	svc.recordLoginAudit("region-user", "auth.login", "success", &LoginRequest{
		ClientIP:  "1.2.3.4",
		UserAgent: "ua-region",
	}, "", "")

	row := lastAuditRow(t, db)
	assert.Contains(t, row["details_json"], "ipRegion")
	assert.Contains(t, row["details_json"], "United States")
}
