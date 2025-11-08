package gamesmeta

import "gorm.io/gorm/clause"

// upsert by business key game_id, not auto ID.
func onConflictByID() clause.Expression {
	return clause.OnConflict{Columns: []clause.Column{{Name: "game_id"}}, UpdateAll: true}
}
