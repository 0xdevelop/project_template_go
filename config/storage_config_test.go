package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/0xdevelop/project_template_go/storage/storage_config"
	"github.com/george012/gtbox/gtbox_orm/gtbox_orm_config"
)

func TestStorageConfigKeepsOldKeys(t *testing.T) {
	previous := GlobalConfig
	t.Cleanup(func() { GlobalConfig = previous })
	for _, testCase := range []struct {
		name    string
		content string
		legacy  bool
	}{
		{
			name:    "legacy.yaml",
			content: "mysql_cfg:\n  dbname: project_template_go\n  dbuser: project_template_go\n  dbpwd: project_template_go_pwd\n  dbaddress: 127.0.0.1\n  dbport: \"3306\"\n  dbtimezone: 0\n",
			legacy:  true,
		},
		{
			name:    "legacy.json",
			content: `{"mysql_cfg":{"dbname":"project_template_go","dbuser":"project_template_go","dbpwd":"project_template_go_pwd","dbaddress":"127.0.0.1","dbport":"3306","dbtimezone":0}}`,
			legacy:  true,
		},
		{
			name:    "legacy.toml",
			content: "[mysql_cfg]\ndbname = 'project_template_go'\ndbuser = 'project_template_go'\ndbpwd = 'project_template_go_pwd'\ndbaddress = '127.0.0.1'\ndbport = 3306\ndbtimezone = 0\n",
			legacy:  true,
		},
		{
			name:    "current.yaml",
			content: "storage_cfg:\n  storage_sqlite:\n    enabled: true\n    path: ./data/project_template_go.db\n  storage_mysql:\n    enabled: false\n    dbname: project_template_go\n    dbuser: project_template_go\n    dbpwd: project_template_go_pwd\n    dbaddress: 10.0.0.2\n    dbport: 3306\n    dbtimezone: 1\n  storage_postgresql:\n    enabled: false\n    dbname: project_template_go\n    dbuser: project_template_go\n    dbpwd: project_template_go_pwd\n    dbaddress: 10.0.0.8\n    dbport: 5432\n    sslmode: require\n  storage_redis:\n    enabled: false\n    addr: 127.0.0.1:6379\n    password: redis-secret\n    db_index: 3\n    database: true\n    prefix: app\n",
		},
		{
			name:    "current.json",
			content: `{"storage_cfg":{"storage_sqlite":{"enabled":true,"path":"./data/project_template_go.db"},"storage_mysql":{"enabled":false,"dbname":"project_template_go","dbuser":"project_template_go","dbpwd":"project_template_go_pwd","dbaddress":"10.0.0.2","dbport":3306,"dbtimezone":1},"storage_postgresql":{"enabled":false,"dbname":"project_template_go","dbuser":"project_template_go","dbpwd":"project_template_go_pwd","dbaddress":"10.0.0.8","dbport":5432,"sslmode":"require"},"storage_redis":{"enabled":false,"addr":"127.0.0.1:6379","password":"redis-secret","db_index":3,"database":true,"prefix":"app"}}}`,
		},
		{
			name:    "current.toml",
			content: "[storage_cfg.storage_sqlite]\nenabled = true\npath = './data/project_template_go.db'\n[storage_cfg.storage_mysql]\nenabled = false\ndbname = 'project_template_go'\ndbuser = 'project_template_go'\ndbpwd = 'project_template_go_pwd'\ndbaddress = '10.0.0.2'\ndbport = 3306\ndbtimezone = 1\n[storage_cfg.storage_postgresql]\nenabled = false\ndbname = 'project_template_go'\ndbuser = 'project_template_go'\ndbpwd = 'project_template_go_pwd'\ndbaddress = '10.0.0.8'\ndbport = 5432\nsslmode = 'require'\n[storage_cfg.storage_redis]\nenabled = false\naddr = '127.0.0.1:6379'\npassword = 'redis-secret'\ndb_index = 3\ndatabase = true\nprefix = 'app'\n",
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			GlobalConfig = nil
			file := filepath.Join(t.TempDir(), testCase.name)
			if err := os.WriteFile(file, []byte(testCase.content), 0600); err != nil {
				t.Fatal(err)
			}
			if err := LoadConfig(file); err != nil {
				t.Fatal(err)
			}
			mysql := GlobalConfig.StorageCfg.Mysql
			if mysql == nil || !mysql.Enabled && testCase.legacy {
				t.Fatalf("mysql was not loaded: %+v", GlobalConfig.StorageCfg)
			}
			if testCase.legacy {
				if mysql.DBName != "project_template_go" || mysql.DBUser != "project_template_go" || mysql.DBPwd != "project_template_go_pwd" || mysql.DBHost != "127.0.0.1" || mysql.DBPort != 3306 || mysql.DBTimeZone != gtbox_orm_config.GTORMTimeZoneUTC || !mysql.Enabled {
					t.Fatalf("legacy mysql values: %+v", mysql.GTORMMysqlConfig)
				}
				return
			}
			if GlobalConfig.StorageCfg.Sqlite == nil || GlobalConfig.StorageCfg.Sqlite.DBFilePath != "./data/project_template_go.db" {
				t.Fatalf("sqlite path: %+v", GlobalConfig.StorageCfg.Sqlite)
			}
			if mysql.DBHost != "10.0.0.2" || mysql.DBPort != 3306 || mysql.DBName != "project_template_go" || mysql.DBPwd != "project_template_go_pwd" || mysql.DBTimeZone != gtbox_orm_config.GTORMTimeZoneShangHai {
				t.Fatalf("mysql values: %+v", mysql.GTORMMysqlConfig)
			}
			postgres := GlobalConfig.StorageCfg.PostgreSQL
			if postgres == nil || postgres.DBHost != "10.0.0.8" || postgres.DBPort != 5432 || postgres.DBSSLMode != "require" || postgres.DBName != "project_template_go" {
				t.Fatalf("postgres values: %+v", postgres)
			}
			redis := GlobalConfig.StorageCfg.Redis
			if redis == nil || redis.Addr != "127.0.0.1:6379" || redis.Pwd != "redis-secret" || redis.SocketBuck != 3 || !redis.Database || redis.Prefix != "app" {
				t.Fatalf("redis values: %+v", redis)
			}
		})
	}
}

func TestStorageLegacyTimeZone(t *testing.T) {
	previous := GlobalConfig
	t.Cleanup(func() { GlobalConfig = previous })
	for _, format := range []struct{ ext, content string }{
		{"yaml", "mysql_cfg:\n  dbname: review\n  dbtimezone: %s\n"},
		{"json", `{"mysql_cfg":{"dbname":"review","dbtimezone":%s}}`},
		{"toml", "[mysql_cfg]\ndbname = 'review'\ndbtimezone = %s\n"},
	} {
		for _, testCase := range []struct {
			value string
			want  gtbox_orm_config.GTORMTimeZone
		}{
			{`"UTC"`, gtbox_orm_config.GTORMTimeZoneUTC},
			{`0`, gtbox_orm_config.GTORMTimeZoneUTC},
			{`"0"`, gtbox_orm_config.GTORMTimeZoneUTC},
			{`"Asia/Shanghai"`, gtbox_orm_config.GTORMTimeZoneShangHai},
			{`"Asia%2FShanghai"`, gtbox_orm_config.GTORMTimeZoneShangHai},
			{`1`, gtbox_orm_config.GTORMTimeZoneShangHai},
			{`"1"`, gtbox_orm_config.GTORMTimeZoneShangHai},
			{`"unknown"`, -1},
			{`2`, -1},
		} {
			t.Run(format.ext+"/"+testCase.value, func(t *testing.T) {
				GlobalConfig = nil
				file := filepath.Join(t.TempDir(), "legacy."+format.ext)
				if err := os.WriteFile(file, []byte(fmt.Sprintf(format.content, testCase.value)), 0600); err != nil {
					t.Fatal(err)
				}
				err := LoadConfig(file)
				if testCase.want < 0 {
					if err == nil || !strings.Contains(err.Error(), "mysql_cfg.dbtimezone") {
						t.Fatalf("invalid time zone must report its configuration key: %v", err)
					}
					return
				}
				if err != nil {
					t.Fatal(err)
				}
				if got := GlobalConfig.StorageCfg.Mysql.DBTimeZone; got != testCase.want {
					t.Fatalf("time zone: got %v, want %v", got, testCase.want)
				}
			})
		}
	}
}

func TestStoragePrimaryConfig(t *testing.T) {
	for _, testCase := range []struct {
		name, primary           string
		mysql, sqlite, postgres bool
		invalid                 bool
	}{
		{name: "single", sqlite: true},
		{name: "mysql", primary: "mysql", mysql: true, sqlite: true, postgres: true},
		{name: "sqlite", primary: "sqlite", mysql: true, sqlite: true, postgres: true},
		{name: "postgresql", primary: "postgresql", mysql: true, sqlite: true, postgres: true},
		{name: "missing", mysql: true, sqlite: true, invalid: true},
		{name: "disabled", primary: "mysql", sqlite: true, invalid: true},
		{name: "two", primary: "mysql,sqlite", mysql: true, sqlite: true, invalid: true},
		{name: "redis", primary: "redis", invalid: true},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			cfg := &storage_config.StorageConfig{
				Primary:    testCase.primary,
				Mysql:      &storage_config.StorageMysqlConfig{Enabled: testCase.mysql},
				Sqlite:     &storage_config.StorageSqliteConfig{Enabled: testCase.sqlite},
				PostgreSQL: &storage_config.StoragePostgresConfig{Enabled: testCase.postgres},
			}
			err := storage_config.StorageConfigValidate(cfg)
			if (err != nil) != testCase.invalid {
				t.Fatalf("invalid=%v, err=%v", testCase.invalid, err)
			}
		})
	}
}

func TestStorageGeneratedConfig(t *testing.T) {
	previous := GlobalConfig
	t.Cleanup(func() { GlobalConfig = previous })
	for _, ext := range []string{"yaml", "json", "toml"} {
		t.Run(ext, func(t *testing.T) {
			GlobalConfig = nil
			file := filepath.Join(t.TempDir(), "generated."+ext)
			if err := SaveConfig(file, generateDefaultConfig()); err != nil {
				t.Fatal(err)
			}
			if err := LoadConfig(file); err != nil {
				t.Fatal(err)
			}
			if GlobalConfig.StorageCfg.Sqlite == nil {
				t.Fatal("generated SQLite configuration is missing")
			}
			if GlobalConfig.StorageCfg.Primary != "sqlite" || !GlobalConfig.StorageCfg.Sqlite.Enabled || GlobalConfig.StorageCfg.Sqlite.DBFilePath != "./data/project_template_go.db" {
				t.Fatalf("generated SQLite configuration: primary=%q enabled=%v dbfilepath=%q path=%q", GlobalConfig.StorageCfg.Primary, GlobalConfig.StorageCfg.Sqlite.Enabled, GlobalConfig.StorageCfg.Sqlite.DBFilePath, GlobalConfig.StorageCfg.Sqlite.Path)
			}
		})
	}
	GlobalConfig = nil
	if err := LoadConfig(filepath.Join("..", "example_files", "config_example.yaml")); err != nil {
		t.Fatalf("published configuration example: %v", err)
	}
}
