package sitesettings

import (
	"strings"
	"testing"

	gsqlite "github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func body(s string) *strings.Reader { return strings.NewReader(s) }

func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(gsqlite.Open(t.TempDir()+"/settings.db"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	return db
}
