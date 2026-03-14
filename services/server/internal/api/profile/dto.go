package profile

// ProfileGetRequest 获取个人资料请求
type ProfileGetRequest struct{}

// ProfileGetResponse 获取个人资料响应
type ProfileGetResponse struct {
	Username string   `json:"username"`
	Nickname string   `json:"nickname"`
	Email    string   `json:"email"`
	Phone    string   `json:"phone"`
	Roles    []string `json:"roles"`
}

// ProfileGamesRequest 获取我的游戏请求
type ProfileGamesRequest struct{}

// ProfileGamesResponse 获取我的游戏响应
type ProfileGamesResponse struct {
	Games []GameInfo `json:"games"`
}

// GameInfo 游戏信息
type GameInfo struct {
	GameID   string `json:"gameId"`
	GameName string `json:"gameName"`
}

// ProfileUpdateRequest 更新个人资料请求
type ProfileUpdateRequest struct {
	Nickname string `json:"nickname"`
	Email    string `json:"email"`
	Phone    string `json:"phone"`
}

// ProfileUpdateResponse 更新个人资料响应
type ProfileUpdateResponse struct {
	Ok bool `json:"ok"`
}

// ChangePasswordRequest 修改密码请求
type ChangePasswordRequest struct {
	OldPassword string `json:"oldPassword" binding:"required"`
	NewPassword string `json:"newPassword" binding:"required,min=6"`
}

// ChangePasswordResponse 修改密码响应
type ChangePasswordResponse struct {
	Ok bool `json:"ok"`
}
