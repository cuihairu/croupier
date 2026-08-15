package users

import (
	"encoding/json"
	"errors"
	"os"

	"golang.org/x/crypto/bcrypt"
)

// User is a local file-based account entry. Password stores a bcrypt hash;
// Salt is kept only for JSON backward compatibility with legacy files and is
// ignored by verification (bcrypt hashes embed their own salt).
type User struct {
	Username  string   `json:"username"`
	Salt      string   `json:"salt,omitempty"`
	Password  string   `json:"password"` // bcrypt hash
	Roles     []string `json:"roles"`
	Perms     []string `json:"perms,omitempty"`
	OTPSecret string   `json:"otpSecret,omitempty"`
}

// HashPassword derives the bcrypt hash to store in a users file.
func HashPassword(password string) (string, error) {
	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hashed), nil
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
	if err := bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(password)); err != nil {
		return User{}, errors.New("invalid credentials")
	}
	return u, nil
}
