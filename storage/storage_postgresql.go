package storage

import (
	"errors"

	"github.com/0xdevelop/project_template_go/storage/storage_config"
	"github.com/george012/gtbox/gtbox_orm/gtbox_orm_postgres"
)

func openPostgreSQL(cfg *storage_config.StoragePostgresConfig) error {
	if cfg == nil {
		return errors.New("storage_postgresql is not configured")
	}
	opened, err := gtbox_orm_postgres.New(cfg.GTORMPostgresConfig)
	if err != nil {
		return err
	}
	globalPostgreSQL = opened
	return nil
}
