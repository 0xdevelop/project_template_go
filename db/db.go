// Package db 是唯一数据库入口。
// 使用模式与引擎无关。换 MySQL / SQLite 只改 OpenStorage 里生效的那一行。
package db

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/0xdevelop/project_template_go/ability/ability_task/ability_task_model"
	"github.com/0xdevelop/project_template_go/ability/ability_user/ability_user_model"
	"github.com/0xdevelop/project_template_go/api/api_auth/api_auth_model"
	"github.com/0xdevelop/project_template_go/config"
	"github.com/george012/gtbox/gtbox_orm/gtbox_orm_config"
	"github.com/george012/gtbox/gtbox_orm/gtbox_orm_mysql"
	"github.com/george012/gtbox/gtbox_orm/gtbox_orm_sqlite"
	mysqlDriver "github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
)

const sqliteFilePath = "./data/project_template_go.db"

var (
	globalSqlite *gtbox_orm_sqlite.GTORMSqlite
	globalMysql  *gtbox_orm_mysql.GTORMMysql
	globalGormDB *gorm.DB
)

// OpenStorage 打开库并记下全局 GORM 实例。换引擎时注释当前行、打开另一行。
func OpenStorage() error {
	return openMysql()
	// return openSqlite()
}

// CloseStorage 关闭当前引擎的连接池。
func CloseStorage() error {
	var err error
	if globalSqlite != nil {
		err = globalSqlite.Close()
		globalSqlite = nil
	}
	if globalMysql != nil {
		if closeErr := globalMysql.Close(); err == nil {
			err = closeErr
		}
		globalMysql = nil
	}
	globalGormDB = nil
	return err
}

// GormDB 返回带 ctx 的全局 GORM 实例。未打开时返回错误。
func GormDB(ctx context.Context) (*gorm.DB, error) {
	if globalGormDB == nil {
		return nil, errors.New("gorm database is not initialized")
	}
	return globalGormDB.WithContext(ctx), nil
}

// IsDuplicateKeyError 判定唯一键冲突：GORM 通用错误、MySQL 1062、SQLite UNIQUE constraint failed。
func IsDuplicateKeyError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return true
	}
	var mysqlError *mysqlDriver.MySQLError
	if errors.As(err, &mysqlError) && mysqlError.Number == 1062 {
		return true
	}
	return strings.Contains(err.Error(), "UNIQUE constraint failed")
}

// GormAutoMigrate 同步全部业务表结构；业务 model 就位后按 Ability 域在此登记。
func GormAutoMigrate() error {
	opened, err := GormDB(context.Background())
	if err != nil {
		return err
	}
	return opened.AutoMigrate(
		&ability_user_model.User{},
		&api_auth_model.AuthVerifyCode{},
		&api_auth_model.AuthSession{},
		&ability_task_model.AsyncTask{},
	)
}

func openSqlite() error {
	if err := os.MkdirAll(filepath.Dir(sqliteFilePath), 0755); err != nil {
		return err
	}
	opened, err := gtbox_orm_sqlite.New(gtbox_orm_sqlite.GTORMSqliteConfig{DBFilePath: sqliteFilePath})
	if err != nil {
		return err
	}
	opened.SqliteDB.Config.NowFunc = func() time.Time { return time.Now().UTC() }
	globalSqlite = opened
	globalGormDB = opened.SqliteDB
	return nil
}

func openMysql() error {
	if config.GlobalConfig == nil || config.GlobalConfig.MysqlCfg == nil {
		return errors.New("mysql_cfg is empty")
	}
	mysqlCfg := config.GlobalConfig.MysqlCfg
	opened, err := gtbox_orm_mysql.New(gtbox_orm_mysql.GTORMMysqlConfig{
		DBUser:     mysqlCfg.DBUser,
		DBPwd:      mysqlCfg.DBPwd,
		DBName:     mysqlCfg.DBName,
		DBHost:     mysqlCfg.DBAddress,
		DBPort:     mysqlCfg.DBPort,
		DBTimeZone: gtbox_orm_config.GTORMTimeZoneUTC,
	})
	if err != nil {
		return err
	}
	opened.MysqlDB.Config.NowFunc = func() time.Time { return time.Now().UTC() }
	globalMysql = opened
	globalGormDB = opened.MysqlDB
	return nil
}
