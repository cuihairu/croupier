package identity

import (
	"context"
	"errors"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"gorm.io/gorm"
)

type fakeAdminValidator struct {
	admins map[string]*model.Admin
}

func (f *fakeAdminValidator) ValidatePassword(ctx context.Context, username, password string) (*model.Admin, error) {
	a, ok := f.admins[username]
	if !ok {
		return nil, errors.New("user not found")
	}
	if password != "correct-password" {
		return nil, errors.New("invalid password")
	}
	return a, nil
}

func TestLocalProvider_Authenticate(t *testing.T) {
	p := NewLocalProvider(&fakeAdminValidator{admins: map[string]*model.Admin{
		"alice": {Model: gorm.Model{ID: 7}, Username: "alice", Nickname: "Alice", Email: "alice@example.com"},
	}})

	ident, err := p.Authenticate(context.Background(), "alice", "correct-password")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ident.Provider != KindLocal || ident.Username != "alice" || ident.Nickname != "Alice" || ident.Email != "alice@example.com" {
		t.Fatalf("unexpected identity: %+v", ident)
	}
}

func TestLocalProvider_InvalidCredentials(t *testing.T) {
	p := NewLocalProvider(&fakeAdminValidator{admins: map[string]*model.Admin{}})

	for _, tc := range []struct {
		name, user, pass string
	}{
		{"unknown user", "nobody", "x"},
		{"wrong password", "alice", "x"},
		{"empty username", "", "x"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := p.Authenticate(context.Background(), tc.user, tc.pass)
			if !errors.Is(err, ErrInvalidCredentials) {
				t.Fatalf("want ErrInvalidCredentials, got %v", err)
			}
		})
	}
}
