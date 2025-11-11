package users

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
)

type User struct {
	Username  string   `json:"username"`
	Salt      string   `json:"salt"`
	Password  string   `json:"password"` // sha256(salt+password) hex
	Roles     []string `json:"roles"`
	Perms     []string `json:"perms,omitempty"`
	OTPSecret string   `json:"otp_secret,omitempty"`
}

type Store struct {
	users map[string]User
}

func Load(path string) (*Store, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var arr []User
	if err := json.Unmarshal(b, &arr); err != nil {
		return nil, err
	}
	m := make(map[string]User, len(arr))
	for _, u := range arr {
		m[u.Username] = u
	}
	return &Store{users: m}, nil
}

func (s *Store) Get(username string) (User, bool) { u, ok := s.users[username]; return u, ok }

func (s *Store) Verify(username, password string) (User, error) {
	u, ok := s.users[username]
	if !ok {
		return User{}, errors.New("user not found")
	}
	h := sha256.Sum256([]byte(u.Salt + password))
	if hex.EncodeToString(h[:]) != u.Password {
		return User{}, errors.New("invalid credentials")
	}
	return u, nil
}
