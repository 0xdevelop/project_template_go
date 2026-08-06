// Package db db/db.go
package db

import (
	"context"
	"errors"

	"github.com/george012/gtbox/gtbox_orm/gtbox_orm_mysql"
	mysqlDriver "github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
)

var (
	GlobalMysqlCtl *gtbox_orm_mysql.GTORMMysql
)

func MysqlDB(ctx context.Context) (*gorm.DB, error) {
	if GlobalMysqlCtl == nil || GlobalMysqlCtl.MysqlDB == nil {
		return nil, errors.New("mysql database is not initialized")
	}
	return GlobalMysqlCtl.MysqlDB.WithContext(ctx), nil
}

func IsDuplicateKeyError(err error) bool {
	var mysqlError *mysqlDriver.MySQLError
	return errors.As(err, &mysqlError) && mysqlError.Number == 1062
}

// MysqlAutoMigrate 同步全部业务表结构；业务 model 就位后按 Ability 域在此登记。
func MysqlAutoMigrate() error {
	db, err := MysqlDB(context.Background())
	if err != nil {
		return err
	}
	return db.AutoMigrate()
}
