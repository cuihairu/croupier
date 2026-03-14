package user

// UserInfo represents user information with roles and contact details.
type UserInfo struct {
	Username string   `json:"username"`
	Roles    []string `json:"roles"`
	Nickname string   `json:"nickname,omitempty"`
	Email    string   `json:"email,omitempty"`
	Phone    string   `json:"phone,omitempty"`
}
