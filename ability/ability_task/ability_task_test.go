package ability_task

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/0xdevelop/project_template_go/ability/ability_task/ability_task_model"
	"github.com/0xdevelop/project_template_go/config"
	"github.com/0xdevelop/project_template_go/storage"
	"github.com/0xdevelop/project_template_go/storage/storage_config"
	"github.com/george012/gtbox/gtbox_orm/gtbox_orm_sqlite"
)

func TestTaskRejectsSqliteWithoutMysql(t *testing.T) {
	previous := config.GlobalConfig
	t.Cleanup(func() {
		_ = storage.CloseStorage()
		config.GlobalConfig = previous
	})
	config.GlobalConfig = &config.FileConfig{StorageCfg: &storage_config.StorageConfig{
		Primary: "sqlite",
		Sqlite: &storage_config.StorageSqliteConfig{
			Enabled:           true,
			GTORMSqliteConfig: gtbox_orm_sqlite.GTORMSqliteConfig{DBFilePath: filepath.Join(t.TempDir(), "app.db")},
		},
	}}
	if err := storage.OpenStorage(); err != nil {
		t.Fatal(err)
	}
	if err := storage.GormAutoMigrate(); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if _, err := AcceptAsyncTask(ctx, "unregistered", map[string]interface{}{}); err == nil || !strings.Contains(err.Error(), "storage_mysql") {
		t.Fatalf("task acceptance must require MySQL: %v", err)
	}
	if _, err := ExecuteNextQueuedTask(ctx); err == nil || !strings.Contains(err.Error(), "storage_mysql") {
		t.Fatalf("task execution must require MySQL: %v", err)
	}
	db, err := storage.GormDB(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var count int64
	if err := db.Model(&ability_task_model.AsyncTask{}).Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("rejected task was written to SQLite: count=%d err=%v", count, err)
	}
}
