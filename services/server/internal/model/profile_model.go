package model

import (
	"context"

	"gorm.io/gorm"
)

// ProfileModel provides helpers for profile-related views.
type ProfileModel struct {
	db *gorm.DB
}

// NewProfileModel creates helper.
func NewProfileModel(db *gorm.DB) *ProfileModel {
	return &ProfileModel{db: db}
}

// ReplacePermissions replaces cached permissions for an admin.
func (m *ProfileModel) ReplacePermissions(ctx context.Context, adminID uint, perms []ProfilePermission) error {
	return m.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("admin_id = ?", adminID).Delete(&ProfilePermission{}).Error; err != nil {
			return err
		}
		for i := range perms {
			perms[i].AdminID = adminID
			if err := tx.Create(&perms[i]).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// ListPermissions returns cached permissions.
func (m *ProfileModel) ListPermissions(ctx context.Context, adminID uint) ([]ProfilePermission, error) {
	var perms []ProfilePermission
	err := m.db.WithContext(ctx).
		Where("admin_id = ?", adminID).
		Find(&perms).Error
	return perms, err
}

// ReplaceGames replaces cached game scopes.
func (m *ProfileModel) ReplaceGames(ctx context.Context, adminID uint, games []ProfileGame) error {
	return m.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("admin_id = ?", adminID).Delete(&ProfileGame{}).Error; err != nil {
			return err
		}
		for i := range games {
			games[i].AdminID = adminID
			if err := tx.Create(&games[i]).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// ListGames returns cached games.
func (m *ProfileModel) ListGames(ctx context.Context, adminID uint) ([]ProfileGame, error) {
	var games []ProfileGame
	err := m.db.WithContext(ctx).
		Where("admin_id = ?", adminID).
		Find(&games).Error
	return games, err
}
