package storage

import (
	"errors"
	"time"

	"github.com/0xdevelop/project_template_go/storage/storage_config"
	"github.com/george012/gtbox/gtbox_orm/gtbox_orm_mysql"
)

func openMysql(cfg *storage_config.StorageMysqlConfig) error {
	if cfg == nil {
		return errors.New("storage_mysql is not configured")
	}
	opened, err := gtbox_orm_mysql.New(cfg.GTORMMysqlConfig)
	if err != nil {
		return err
	}
	opened.MysqlDB.Config.NowFunc = func() time.Time { return time.Now().UTC() }
	globalMysql = opened
	return nil
}
