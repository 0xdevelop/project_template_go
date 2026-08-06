// Package db_config db/db_config/db_config.go
package db_config

type MysqlConfig struct {
	DBName     string
	DBUser     string
	DBPwd      string
	DBAddress  string
	DBPort     int
	DBTimeZone string
}
