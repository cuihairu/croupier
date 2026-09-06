package tasks

import (
	"context"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// TestStore_AppendEventNextSeqError covers the branch where the event
// sequence query fails before an event can be appended.
func TestStore_AppendEventNextSeqError(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.TaskRun{}, &model.TaskEvent{}))
	require.NoError(t, db.Migrator().DropTable("task_events"))

	store := NewStore(model.NewTaskRunModel(db), model.NewTaskEventModel(db))
	err = store.AppendEvent(context.Background(), "task-err", EventProgress, 10, "halfway", []byte(`{"ok":true}`))
	assert.Error(t, err)
}
