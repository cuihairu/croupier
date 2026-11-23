// AUD-039 回归：对象列表读取面挂服务层权限门禁。
//
// 根因：AUD-016 为存储管理面的写操作（upload/delete/batch-delete/目录）与
// 签名链接加了 requireStoragePermission，但 ListObjects 漏挂——任何登录用户
// 即可枚举全库对象清单（含 backups/、releases/ 等平台维护对象名），泄露平台
// 内部对象命名与发布产物存在性。修复为列表读取面与写面同一粗粒度门禁
// （admin:all | uploads:write）；前缀级 ABAC 留待后续拆分。
package storage

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestStorage_Aud039_ZeroRoleDeniedOnList 验证零角色账号列表被拒（403
// permission_denied）；超管对照放行，证明拒绝是权限不足而非「资源缺失」。
func TestStorage_Aud039_ZeroRoleDeniedOnList(t *testing.T) {
	svc := NewService(setupSvcCtxWithStore(t))

	_, err := svc.ListObjects(viewerCtx(), &ListObjectsRequest{})
	assertForbidden(t, err, "permission_denied")

	// 超管对照：数据面可读。
	resp, err := svc.ListObjects(authCtx(), &ListObjectsRequest{})
	require.NoError(t, err)
	assert.NotNil(t, resp)
}

// TestStorage_Aud039_UploadsWriteGrantsList 验证仅持 uploads:write 的操作员
// 列表放行（读写同域粗粒度权限一致），且能看到自己上传的对象。
func TestStorage_Aud039_UploadsWriteGrantsList(t *testing.T) {
	svc := NewService(setupSvcCtxWithStore(t))

	up, err := svc.UploadObject(operatorCtx(), uploadReq("uploads/listable.txt", "listable"))
	require.NoError(t, err)
	assert.Equal(t, "uploads/listable.txt", up.Path)

	resp, err := svc.ListObjects(operatorCtx(), &ListObjectsRequest{Prefix: "uploads/"})
	require.NoError(t, err)
	assert.NotNil(t, resp)

	found := false
	for _, obj := range resp.Objects {
		if obj.Key == "uploads/listable.txt" {
			found = true
			break
		}
	}
	assert.True(t, found, "uploads:write 操作员列表必须看到已上传对象")
}
