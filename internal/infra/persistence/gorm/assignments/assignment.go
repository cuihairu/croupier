package assignments

import (
	"encoding/json"
	"gorm.io/gorm"
	"time"
)

// Assignment represents function assignments for a game/environment combination
type Assignment struct {
	ID          uint   `gorm:"primaryKey"`
	GameID      string `gorm:"column:game_id;index:idx_game_env,unique"`
	Environment string `gorm:"column:environment;index:idx_game_env,unique"`
	Functions   string `gorm:"column:functions;type:text"` // JSON array of function IDs
	UpdatedBy   string `gorm:"column:updated_by"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (Assignment) TableName() string {
	return "assignments"
}

// Store implements assignment persistence using GORM
type Store struct {
	db *gorm.DB
}

func NewStore(db *gorm.DB) *Store {
	return &Store{db: db}
}

// AutoMigrate creates the assignments table
func (s *Store) AutoMigrate() error {
	return s.db.AutoMigrate(&Assignment{})
}

// Get retrieves assignments for a specific game/environment
func (s *Store) Get(gameID, env string) ([]string, error) {
	var assignment Assignment
	err := s.db.Where("game_id = ? AND environment = ?", gameID, env).First(&assignment).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return []string{}, nil
		}
		return nil, err
	}

	var functions []string
	if assignment.Functions != "" {
		if err := json.Unmarshal([]byte(assignment.Functions), &functions); err != nil {
			return nil, err
		}
	}
	return functions, nil
}

// Set updates assignments for a specific game/environment
func (s *Store) Set(gameID, env string, functions []string, updatedBy string) error {
	functionsJSON, err := json.Marshal(functions)
	if err != nil {
		return err
	}

	assignment := Assignment{
		GameID:      gameID,
		Environment: env,
		Functions:   string(functionsJSON),
		UpdatedBy:   updatedBy,
	}

	return s.db.Model(&Assignment{}).
		Where("game_id = ? AND environment = ?", gameID, env).
		Assign(assignment).
		FirstOrCreate(&assignment).Error
}

// List retrieves all assignments with optional filtering
func (s *Store) List(gameID, env string) (map[string][]string, error) {
	var assignments []Assignment
	query := s.db

	if gameID != "" {
		query = query.Where("game_id = ?", gameID)
	}
	if env != "" {
		query = query.Where("environment = ?", env)
	}

	if err := query.Find(&assignments).Error; err != nil {
		return nil, err
	}

	result := make(map[string][]string)
	for _, assignment := range assignments {
		key := assignment.GameID + "|" + assignment.Environment
		var functions []string
		if assignment.Functions != "" {
			if err := json.Unmarshal([]byte(assignment.Functions), &functions); err != nil {
				continue // Skip malformed entries
			}
		}
		result[key] = functions
	}

	return result, nil
}

// Delete removes assignments for a specific game/environment
func (s *Store) Delete(gameID, env string) error {
	return s.db.Where("game_id = ? AND environment = ?", gameID, env).Delete(&Assignment{}).Error
}