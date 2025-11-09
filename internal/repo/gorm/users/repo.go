package usersgorm

import (
	"context"
	"errors"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"strings"
)

type Repo struct{ db *gorm.DB }

func New(db *gorm.DB) *Repo { return &Repo{db: db} }

// Users
func (r *Repo) CreateUser(ctx context.Context, u *UserAccount) error {
	return r.db.WithContext(ctx).Create(u).Error
}
func (r *Repo) UpdateUser(ctx context.Context, u *UserAccount) error {
	return r.db.WithContext(ctx).Save(u).Error
}
func (r *Repo) DeleteUser(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&UserAccount{}, id).Error
}
func (r *Repo) GetUserByUsername(ctx context.Context, username string) (*UserAccount, error) {
	var ur UserAccount
	if err := r.db.WithContext(ctx).Where("username = ?", username).First(&ur).Error; err != nil {
		return nil, err
	}
	return &ur, nil
}
func (r *Repo) ListUsers(ctx context.Context) ([]*UserAccount, error) {
	var arr []*UserAccount
	if err := r.db.WithContext(ctx).Order("id DESC").Find(&arr).Error; err != nil {
		return nil, err
	}
	return arr, nil
}

func (r *Repo) SetPassword(ctx context.Context, userID uint, plain string) error {
	if strings.TrimSpace(plain) == "" {
		return errors.New("empty password")
	}
	h, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return r.db.WithContext(ctx).Model(&UserAccount{}).Where("id = ?", userID).Update("password_hash", string(h)).Error
}

func (r *Repo) Verify(ctx context.Context, username, plain string) (*UserAccount, error) {
	u, err := r.GetUserByUsername(ctx, username)
	if err != nil {
		return nil, err
	}
	if u.PasswordHash == "" {
		return nil, errors.New("password not set")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(plain)); err != nil {
		return nil, errors.New("invalid credentials")
	}
	if !u.Active {
		return nil, errors.New("user disabled")
	}
	return u, nil
}

// Roles and permissions
func (r *Repo) CreateRole(ctx context.Context, role *RoleRecord) error {
	return r.db.WithContext(ctx).Create(role).Error
}
func (r *Repo) DeleteRole(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&RoleRecord{}, id).Error
}
func (r *Repo) ListRoles(ctx context.Context) ([]*RoleRecord, error) {
	var arr []*RoleRecord
	if err := r.db.WithContext(ctx).Order("id DESC").Find(&arr).Error; err != nil {
		return nil, err
	}
	return arr, nil
}
func (r *Repo) AddUserRole(ctx context.Context, userID, roleID uint) error {
	return r.db.WithContext(ctx).Create(&UserRoleRecord{UserID: userID, RoleID: roleID}).Error
}
func (r *Repo) RemoveUserRole(ctx context.Context, userID, roleID uint) error {
	return r.db.WithContext(ctx).Where("user_id = ? AND role_id = ?", userID, roleID).Delete(&UserRoleRecord{}).Error
}
func (r *Repo) ListUserRoles(ctx context.Context, userID uint) ([]*RoleRecord, error) {
	var roles []*RoleRecord
	if err := r.db.WithContext(ctx).Raw("SELECT r.* FROM role_records r JOIN user_role_records ur ON r.id=ur.role_id WHERE ur.user_id=?", userID).Scan(&roles).Error; err != nil {
		return nil, err
	}
	return roles, nil
}
func (r *Repo) GrantRolePerm(ctx context.Context, roleID uint, perm string) error {
	return r.db.WithContext(ctx).Create(&RolePermRecord{RoleID: roleID, Perm: perm}).Error
}
func (r *Repo) RevokeRolePerm(ctx context.Context, roleID uint, perm string) error {
	return r.db.WithContext(ctx).Where("role_id=? AND perm=?", roleID, perm).Delete(&RolePermRecord{}).Error
}
func (r *Repo) ListRolePerms(ctx context.Context, roleID uint) ([]string, error) {
	var perms []string
	if err := r.db.WithContext(ctx).Model(&RolePermRecord{}).Where("role_id=?", roleID).Pluck("perm", &perms).Error; err != nil {
		return nil, err
	}
	return perms, nil
}
func (r *Repo) BuildPolicySnapshot(ctx context.Context) (map[string][]string, error) {
	// roleName -> perms
	type row struct {
		Name string
		Perm string
	}
	var rows []row
	if err := r.db.WithContext(ctx).Raw("SELECT r.name as name, rp.perm as perm FROM role_records r JOIN role_perm_records rp ON r.id = rp.role_id").Scan(&rows).Error; err != nil {
		return nil, err
	}
	out := map[string][]string{}
	for _, x := range rows {
		out[x.Name] = append(out[x.Name], x.Perm)
	}
	return out, nil
}

// Game scopes for users
func (r *Repo) ListUserGameIDs(ctx context.Context, userID uint) ([]uint, error) {
	var ids []uint
	if err := r.db.WithContext(ctx).Model(&UserGameScope{}).Where("user_id=?", userID).Pluck("game_id", &ids).Error; err != nil {
		return nil, err
	}
	return ids, nil
}

// ListUserGameEnvs lists allowed envs for a user under a game. Empty means unrestricted.
func (r *Repo) ListUserGameEnvs(ctx context.Context, userID, gameID uint) ([]string, error) {
    var envs []string
    if err := r.db.WithContext(ctx).Model(&UserGameEnvScope{}).Where("user_id=? AND game_id=?", userID, gameID).Pluck("env", &envs).Error; err != nil {
        return nil, err
    }
    return envs, nil
}

// ReplaceUserGameEnvs replaces env scopes for a user under a game. Empty envs clears scopes (unrestricted envs).
func (r *Repo) ReplaceUserGameEnvs(ctx context.Context, userID, gameID uint, envs []string) error {
    return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
        if err := tx.Where("user_id=? AND game_id=?", userID, gameID).Delete(&UserGameEnvScope{}).Error; err != nil {
            return err
        }
        seen := map[string]struct{}{}
        for _, e := range envs {
            e = strings.TrimSpace(e)
            if e == "" { continue }
            if _, ok := seen[e]; ok { continue }
            seen[e] = struct{}{}
            if err := tx.Create(&UserGameEnvScope{UserID: userID, GameID: gameID, Env: e}).Error; err != nil {
                return err
            }
        }
        return nil
    })
}

// ReplaceUserGameIDs replaces all game scopes for the user with the provided list
func (r *Repo) ReplaceUserGameIDs(ctx context.Context, userID uint, gameIDs []uint) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("user_id=?", userID).Delete(&UserGameScope{}).Error; err != nil {
			return err
		}
		// Deduplicate
		seen := map[uint]struct{}{}
		for _, gid := range gameIDs {
			if gid == 0 {
				continue
			}
			if _, ok := seen[gid]; ok {
				continue
			}
			seen[gid] = struct{}{}
			if err := tx.Create(&UserGameScope{UserID: userID, GameID: gid}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
