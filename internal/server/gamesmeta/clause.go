package gamesmeta

import "gorm.io/gorm/clause"

func onConflictByID() any { return clause.OnConflict{UpdateAll: true} }

