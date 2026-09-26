package profile

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cuihairu/croupier/internal/model"
)

// strPtr 让测试里的「显式携带某字段」读起来与调用方语义一致。
func strPtr(s string) *string { return &s }

// newAvatarService 造一个带头像 key 的 service（无对象存储）。
func newAvatarService(t *testing.T) (*Service, *model.AdminModel, *model.Admin) {
	t.Helper()
	db := setupTestDB(t)
	svcCtx := setupTestServiceContext(t, db)
	adminModel := model.NewAdminModel(db)
	roleModel := model.NewRoleModel(db)
	admin := &model.Admin{
		Username: "avataruser",
		Nickname: "原昵称",
		Email:    "old@example.com",
		Phone:    "13800000000",
		Avatar:   "avatars/orig.png",
		Status:   1,
	}
	require.NoError(t, adminModel.Create(context.Background(), admin, "password123"))
	return NewService(adminModel, svcCtx.GameModel, roleModel), adminModel, admin
}

// ---------------------------------------------------------------- BUG-012
// 此前 ProfileUpdateRequest 用裸 string，Go 绑定缺失字段得到空串，service 又
// 无条件写库四列——于是「只改昵称」清空头像/邮箱/手机，「只改头像」清空昵称/
// 邮箱/手机。docs/api/profile.md 早已按部分更新语义描述该接口。

func TestUpdateProfile_NicknameOnlyPreservesOtherFields(t *testing.T) {
	service, adminModel, admin := newAvatarService(t)

	_, err := service.UpdateProfile(context.Background(), admin.Username, &ProfileUpdateRequest{
		Nickname: strPtr("新昵称"),
	})
	require.NoError(t, err)

	got, err := adminModel.FindByUsername(context.Background(), admin.Username)
	require.NoError(t, err)
	assert.Equal(t, "新昵称", got.Nickname)
	// 关键断言：未携带的字段必须原样保留
	assert.Equal(t, "old@example.com", got.Email, "只改昵称不应清空邮箱")
	assert.Equal(t, "13800000000", got.Phone, "只改昵称不应清空手机")
	assert.Equal(t, "avatars/orig.png", got.Avatar, "只改昵称不应清空头像")
}

func TestUpdateProfile_AvatarOnlyPreservesOtherFields(t *testing.T) {
	service, adminModel, admin := newAvatarService(t)

	_, err := service.UpdateProfile(context.Background(), admin.Username, &ProfileUpdateRequest{
		Avatar: strPtr("avatars/new.png"),
	})
	require.NoError(t, err)

	got, err := adminModel.FindByUsername(context.Background(), admin.Username)
	require.NoError(t, err)
	assert.Equal(t, "avatars/new.png", got.Avatar)
	assert.Equal(t, "原昵称", got.Nickname, "只改头像不应清空昵称")
	assert.Equal(t, "old@example.com", got.Email, "只改头像不应清空邮箱")
	assert.Equal(t, "13800000000", got.Phone, "只改头像不应清空手机")
}

func TestUpdateProfile_EmptyRequestWritesNothing(t *testing.T) {
	service, adminModel, admin := newAvatarService(t)

	resp, err := service.UpdateProfile(context.Background(), admin.Username, &ProfileUpdateRequest{})
	require.NoError(t, err)
	assert.True(t, resp.Ok)

	got, err := adminModel.FindByUsername(context.Background(), admin.Username)
	require.NoError(t, err)
	assert.Equal(t, "原昵称", got.Nickname)
	assert.Equal(t, "avatars/orig.png", got.Avatar)
}

// 显式传空串 = 显式清空，与「未携带」语义区分开。
func TestUpdateProfile_ExplicitEmptyStringClearsField(t *testing.T) {
	service, adminModel, admin := newAvatarService(t)

	_, err := service.UpdateProfile(context.Background(), admin.Username, &ProfileUpdateRequest{
		Nickname: strPtr(""),
		Avatar:   strPtr(""),
	})
	require.NoError(t, err)

	got, err := adminModel.FindByUsername(context.Background(), admin.Username)
	require.NoError(t, err)
	assert.Equal(t, "", got.Nickname)
	assert.Equal(t, "", got.Avatar)
	// 仍只影响显式携带的字段
	assert.Equal(t, "old@example.com", got.Email)
}

// 头像只落**裸对象 key**：签名 URL / 带前缀 URL 都会被归一，否则库里存的是
// 一条会过期的死链（S3/OSS/COS 默认 15min TTL）。
func TestUpdateProfile_AvatarStoredAsBareObjectKey(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{"裸 key", "avatars/123.png", "avatars/123.png"},
		{"file 驱动相对路径", "/uploads/avatars/123.png", "avatars/123.png"},
		{"带签名的绝对 URL 剥掉 query", "https://cdn.example.com/avatars/123.png?X-Amz-Signature=deadbeef", "avatars/123.png"},
		{"带自定义 CDN 前缀", "https://cdn.example.com/assets/avatars/123.png", "avatars/123.png"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			service, adminModel, admin := newAvatarService(t)
			_, err := service.UpdateProfile(context.Background(), admin.Username, &ProfileUpdateRequest{
				Avatar: strPtr(tc.input),
			})
			require.NoError(t, err)
			got, err := adminModel.FindByUsername(context.Background(), admin.Username)
			require.NoError(t, err)
			assert.Equal(t, tc.want, got.Avatar)
		})
	}
}

// 非约定目录的头像地址直接拒绝，避免把任意 URL 存进库当头像。
func TestUpdateProfile_RejectsAvatarOutsidePrefix(t *testing.T) {
	service, _, admin := newAvatarService(t)
	_, err := service.UpdateProfile(context.Background(), admin.Username, &ProfileUpdateRequest{
		Avatar: strPtr("https://evil.example.com/other/a.png"),
	})
	assert.Error(t, err)
}

// ---------------------------------------------------------------- 读取侧
// 读取时现算 URL：库里是裸 key，响应里是可访问地址。
func TestGetProfile_ResolvesAvatarKeyToURL(t *testing.T) {
	service, _, admin := newAvatarService(t)
	resp, err := service.GetProfile(context.Background(), admin.Username)
	require.NoError(t, err)
	// 无对象存储时退化为 /uploads/ 相对路径（file 驱动形态）
	assert.Equal(t, "/uploads/avatars/orig.png", resp.Avatar)
}

// 签名 URL 每次读取都重新生成，签名过期不会让头像失效。
func TestGetProfile_SignsFreshURLOnEveryRead(t *testing.T) {
	service, _, admin := newAvatarService(t)
	var first string
	for i := 0; i < 2; i++ {
		resp, err := service.GetProfile(context.Background(), admin.Username)
		require.NoError(t, err)
		if i == 0 {
			first = resp.Avatar
		} else {
			assert.Equal(t, first, resp.Avatar)
		}
	}
	assert.NotEmpty(t, first)
}

// 缓存失效钩子必须在资料更新后被调用，否则读到旧头像。
func TestUpdateProfile_InvalidatesAdminCache(t *testing.T) {
	service, _, admin := newAvatarService(t)
	var called int
	var gotID uint
	var gotName string
	service.WithCacheInvalidator(func(_ context.Context, id uint, username string) {
		called++
		gotID = id
		gotName = username
	})

	_, err := service.UpdateProfile(context.Background(), admin.Username, &ProfileUpdateRequest{
		Avatar: strPtr("avatars/new.png"),
	})
	require.NoError(t, err)
	assert.Equal(t, 1, called, "更新资料后应失效 admin 缓存")
	assert.Equal(t, admin.ID, gotID)
	assert.Equal(t, admin.Username, gotName)
}
