package model

import "gorm.io/gorm/clause"

// upsertAllColumns returns a gorm OnConflict clause updating all columns.
func upsertAllColumns() clause.Expression {
	return clause.OnConflict{
		UpdateAll: true,
	}
}
