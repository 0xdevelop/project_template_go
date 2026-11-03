package storage

import (
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/0xdevelop/project_template_go/storage/storage_config"
	"github.com/george012/gtbox/gtbox_orm/gtbox_orm_sqlite"
)

func openSqlite(cfg *storage_config.StorageSqliteConfig) error {
	if cfg == nil {
		return errors.New("storage_sqlite is not configured")
	}
	if err := os.MkdirAll(filepath.Dir(cfg.DBFilePath), 0755); err != nil {
		return err
	}
	opened, err := gtbox_orm_sqlite.New(cfg.GTORMSqliteConfig)
	if err != nil {
		return err
	}
	opened.SqliteDB.Config.NowFunc = func() time.Time { return time.Now().UTC() }
	globalSqlite = opened
	return nil
}
