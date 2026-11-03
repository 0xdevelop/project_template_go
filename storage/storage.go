// Package storage 按配置打开已启用的存储。关系库和 Redis 可以同时开。
package storage

import (
	"context"
	"errors"
	"strings"
	"sync"

	"github.com/0xdevelop/project_template_go/ability/ability_task/ability_task_model"
	"github.com/0xdevelop/project_template_go/ability/ability_user/ability_user_model"
	"github.com/0xdevelop/project_template_go/api/api_auth/api_auth_model"
	"github.com/0xdevelop/project_template_go/config"
	"github.com/0xdevelop/project_template_go/storage/storage_config"
	"github.com/george012/gtbox/gtbox_orm/gtbox_orm_mysql"
	"github.com/george012/gtbox/gtbox_orm/gtbox_orm_postgres"
	"github.com/george012/gtbox/gtbox_orm/gtbox_orm_sqlite"
	"github.com/george012/gtbox/gtbox_redis"
	mysqlDriver "github.com/go-sql-driver/mysql"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
)

var (
	storageMutex     sync.RWMutex
	storagePrimaryDB *gorm.DB
	globalSqlite     *gtbox_orm_sqlite.GTORMSqlite
	globalMysql      *gtbox_orm_mysql.GTORMMysql
	globalPostgreSQL *gtbox_orm_postgres.GTORMPostgres
	globalRedis      *gtbox_redis.GTRedis
)

// OpenStorage 打开 storage_cfg 里每一个 enabled 的引擎。
func OpenStorage() error {
	storageMutex.Lock()
	defer storageMutex.Unlock()
	if globalSqlite != nil || globalMysql != nil || globalPostgreSQL != nil || globalRedis != nil {
		return errors.New("storage is already initialized; close it before reopening")
	}
	cfg := config.GlobalConfig
	if cfg == nil || cfg.StorageCfg == nil {
		return errors.New("storage_cfg is not configured")
	}
	storageCfg := cfg.StorageCfg
	if err := storage_config.StorageConfigValidate(storageCfg); err != nil {
		return err
	}
	if storageCfg.Sqlite != nil && storageCfg.Sqlite.Enabled {
		if err := openSqlite(storageCfg.Sqlite); err != nil {
			_ = closeStorageLocked()
			return err
		}
	}
	if storageCfg.Mysql != nil && storageCfg.Mysql.Enabled {
		if err := openMysql(storageCfg.Mysql); err != nil {
			_ = closeStorageLocked()
			return err
		}
	}
	if storageCfg.PostgreSQL != nil && storageCfg.PostgreSQL.Enabled {
		if err := openPostgreSQL(storageCfg.PostgreSQL); err != nil {
			_ = closeStorageLocked()
			return err
		}
	}
	if storageCfg.Redis != nil && storageCfg.Redis.Enabled {
		if err := openRedis(storageCfg.Redis); err != nil {
			_ = closeStorageLocked()
			return err
		}
	}
	if len(relationalDBs()) == 0 && !redisIsDatabase(storageCfg) {
		_ = closeStorageLocked()
		return errors.New("storage_cfg has no enabled database")
	}
	switch storageCfg.Primary {
	case "mysql":
		storagePrimaryDB = globalMysql.MysqlDB
	case "sqlite":
		storagePrimaryDB = globalSqlite.SqliteDB
	case "postgresql":
		storagePrimaryDB = globalPostgreSQL.PostgresDB
	default:
		if opened := relationalDBs(); len(opened) == 1 {
			storagePrimaryDB = opened[0]
		}
	}
	return nil
}

// CloseStorage 关闭已经打开的连接。
func CloseStorage() error {
	storageMutex.Lock()
	defer storageMutex.Unlock()
	return closeStorageLocked()
}

func closeStorageLocked() error {
	storagePrimaryDB = nil
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
	if globalPostgreSQL != nil {
		if closeErr := globalPostgreSQL.Close(); err == nil {
			err = closeErr
		}
		globalPostgreSQL = nil
	}
	if globalRedis != nil {
		if closeErr := globalRedis.Close(); err == nil {
			err = closeErr
		}
		globalRedis = nil
	}
	return err
}

// GormDB 返回主关系库；单关系库未指定 primary 时使用该库。
func GormDB(ctx context.Context) (*gorm.DB, error) {
	storageMutex.RLock()
	defer storageMutex.RUnlock()
	if storagePrimaryDB == nil {
		return nil, errors.New("relational storage is not initialized")
	}
	return storagePrimaryDB.WithContext(ctx), nil
}

func SqliteDB(ctx context.Context) (*gorm.DB, error) {
	storageMutex.RLock()
	defer storageMutex.RUnlock()
	if globalSqlite == nil {
		return nil, errors.New("storage_sqlite is not enabled")
	}
	return globalSqlite.SqliteDB.WithContext(ctx), nil
}

func MysqlDB(ctx context.Context) (*gorm.DB, error) {
	storageMutex.RLock()
	defer storageMutex.RUnlock()
	if globalMysql == nil {
		return nil, errors.New("storage_mysql is not enabled")
	}
	return globalMysql.MysqlDB.WithContext(ctx), nil
}

func PostgreSQLDB(ctx context.Context) (*gorm.DB, error) {
	storageMutex.RLock()
	defer storageMutex.RUnlock()
	if globalPostgreSQL == nil {
		return nil, errors.New("storage_postgresql is not enabled")
	}
	return globalPostgreSQL.PostgresDB.WithContext(ctx), nil
}

// IsDuplicateKeyError 判定唯一键冲突。
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
	var pgError *pgconn.PgError
	if errors.As(err, &pgError) && pgError.Code == "23505" {
		return true
	}
	return strings.Contains(err.Error(), "UNIQUE constraint failed")
}

// GormAutoMigrate 给每个已打开的关系库同步业务表。
func GormAutoMigrate() error {
	storageMutex.RLock()
	defer storageMutex.RUnlock()
	opened := relationalDBs()
	if len(opened) == 0 {
		if cfg := config.GlobalConfig; globalRedis != nil && cfg != nil && redisIsDatabase(cfg.StorageCfg) {
			return nil
		}
		return errors.New("relational storage is not initialized")
	}
	for _, openedDB := range opened {
		if err := openedDB.AutoMigrate(
			&ability_user_model.User{},
			&api_auth_model.AuthVerifyCode{},
			&api_auth_model.AuthSession{},
			&ability_task_model.AsyncTask{},
		); err != nil {
			return err
		}
	}
	return nil
}

func relationalDBs() []*gorm.DB {
	var opened []*gorm.DB
	if globalSqlite != nil {
		opened = append(opened, globalSqlite.SqliteDB)
	}
	if globalMysql != nil {
		opened = append(opened, globalMysql.MysqlDB)
	}
	if globalPostgreSQL != nil {
		opened = append(opened, globalPostgreSQL.PostgresDB)
	}
	return opened
}

func redisIsDatabase(cfg *storage_config.StorageConfig) bool {
	return cfg != nil && cfg.Redis != nil && cfg.Redis.Enabled && cfg.Redis.Database
}
