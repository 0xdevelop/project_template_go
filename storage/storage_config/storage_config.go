package storage_config

import (
	"errors"
	"strconv"
	"strings"

	"github.com/george012/gtbox/gtbox_orm/gtbox_orm_config"
	"github.com/george012/gtbox/gtbox_orm/gtbox_orm_mysql"
	"github.com/george012/gtbox/gtbox_orm/gtbox_orm_postgres"
	"github.com/george012/gtbox/gtbox_orm/gtbox_orm_sqlite"
	"github.com/george012/gtbox/gtbox_redis"
	"github.com/goccy/go-json"
	"github.com/goccy/go-yaml"
)

// StorageMysqlConfig 只保留启用开关，连接参数用 SDK 配置。
type StorageMysqlConfig struct {
	Enabled                          bool   `yaml:"enabled" json:"enabled" toml:"enabled"`
	DBAddress                        string `yaml:"dbaddress" json:"dbaddress" toml:"dbaddress"`
	gtbox_orm_mysql.GTORMMysqlConfig `yaml:",inline"`
}

// StorageSqliteConfig 只保留启用开关。path 是已有配置里的库文件键。
type StorageSqliteConfig struct {
	Enabled                            bool   `yaml:"enabled" json:"enabled" toml:"enabled"`
	Path                               string `yaml:"path" json:"path" toml:"path"`
	gtbox_orm_sqlite.GTORMSqliteConfig `yaml:",inline"`
}

// StoragePostgresConfig 只保留启用开关。dbaddress、sslmode 是已有配置键。
type StoragePostgresConfig struct {
	Enabled                                bool   `yaml:"enabled" json:"enabled" toml:"enabled"`
	DBAddress                              string `yaml:"dbaddress" json:"dbaddress" toml:"dbaddress"`
	SSLMode                                string `yaml:"sslmode" json:"sslmode" toml:"sslmode"`
	gtbox_orm_postgres.GTORMPostgresConfig `yaml:",inline"`
}

// StorageRedisConfig 保留启用、是否当库、前缀。password、db_index 是已有配置键。
type StorageRedisConfig struct {
	Enabled                 bool   `yaml:"enabled" json:"enabled" toml:"enabled"`
	Database                bool   `yaml:"database" json:"database" toml:"database"`
	Prefix                  string `yaml:"prefix" json:"prefix" toml:"prefix"`
	Password                string `yaml:"password" json:"password" toml:"password"`
	DBIndex                 *int   `yaml:"db_index" json:"db_index" toml:"db_index"`
	gtbox_redis.RedisConfig `yaml:",inline"`
}

type StorageConfig struct {
	Primary    string                 `yaml:"primary" json:"primary" toml:"primary" comment:"主关系库：mysql、sqlite、postgresql；多关系库时必填"`
	Sqlite     *StorageSqliteConfig   `yaml:"storage_sqlite" json:"storage_sqlite" toml:"storage_sqlite" comment:"SQLite"`
	Mysql      *StorageMysqlConfig    `yaml:"storage_mysql" json:"storage_mysql" toml:"storage_mysql" comment:"MySQL"`
	PostgreSQL *StoragePostgresConfig `yaml:"storage_postgresql" json:"storage_postgresql" toml:"storage_postgresql" comment:"PostgreSQL"`
	Redis      *StorageRedisConfig    `yaml:"storage_redis" json:"storage_redis" toml:"storage_redis" comment:"Redis"`
}

// LegacyMysqlConfig 是根上的 mysql_cfg。只接已有键，打开前并进 SDK 配置。
type LegacyMysqlConfig struct {
	DBName     string                     `yaml:"dbname" json:"dbname" toml:"dbname"`
	DBUser     string                     `yaml:"dbuser" json:"dbuser" toml:"dbuser"`
	DBPwd      string                     `yaml:"dbpwd" json:"dbpwd" toml:"dbpwd"`
	DBAddress  string                     `yaml:"dbaddress" json:"dbaddress" toml:"dbaddress"`
	DBPort     legacyPort                 `yaml:"dbport" json:"dbport" toml:"dbport"`
	DBTimeZone storageLegacyMysqlTimeZone `yaml:"dbtimezone" json:"dbtimezone" toml:"dbtimezone"`
}

type storageLegacyMysqlTimeZone gtbox_orm_config.GTORMTimeZone

func (zone *storageLegacyMysqlTimeZone) UnmarshalText(data []byte) error {
	switch strings.TrimSpace(string(data)) {
	case "", "UTC", "0":
		*zone = storageLegacyMysqlTimeZone(gtbox_orm_config.GTORMTimeZoneUTC)
	case "Asia/Shanghai", "Asia%2FShanghai", "1":
		*zone = storageLegacyMysqlTimeZone(gtbox_orm_config.GTORMTimeZoneShangHai)
	default:
		return errors.New("mysql_cfg.dbtimezone must be UTC/0 or Asia/Shanghai/1")
	}
	return nil
}

func (zone *storageLegacyMysqlTimeZone) UnmarshalJSON(data []byte) error {
	value := strings.TrimSpace(string(data))
	if value == "null" {
		return nil
	}
	if strings.HasPrefix(value, `"`) {
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
	}
	return zone.UnmarshalText([]byte(value))
}

type legacyPort int

func (p *legacyPort) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var text string
	if err := unmarshal(&text); err == nil {
		return p.parse(text)
	}
	var number int
	if err := unmarshal(&number); err != nil {
		return err
	}
	*p = legacyPort(number)
	return nil
}

func (p *legacyPort) UnmarshalJSON(data []byte) error {
	text := strings.Trim(string(data), `"`)
	return p.parse(text)
}

func (p *legacyPort) parse(text string) error {
	text = strings.TrimSpace(text)
	if text == "" || text == "null" {
		return nil
	}
	number, err := strconv.Atoi(text)
	if err != nil {
		return err
	}
	*p = legacyPort(number)
	return nil
}

func Default(sqlitePath string) *StorageConfig {
	return &StorageConfig{
		Primary: "sqlite",
		Sqlite: &StorageSqliteConfig{
			Enabled:           true,
			GTORMSqliteConfig: gtbox_orm_sqlite.GTORMSqliteConfig{DBFilePath: sqlitePath},
		},
		Mysql: &StorageMysqlConfig{
			GTORMMysqlConfig: gtbox_orm_mysql.GTORMMysqlConfig{
				DBName: "test_db",
				DBUser: "test_db_user",
				DBPwd:  "test_db_pwd",
				DBHost: "127.0.0.1",
				DBPort: 3306,
			},
		},
		PostgreSQL: &StoragePostgresConfig{
			GTORMPostgresConfig: gtbox_orm_postgres.GTORMPostgresConfig{
				DBName:    "test_db",
				DBUser:    "test_db_user",
				DBPwd:     "test_db_pwd",
				DBHost:    "127.0.0.1",
				DBPort:    5432,
				DBSSLMode: "disable",
			},
		},
		Redis: &StorageRedisConfig{
			RedisConfig: gtbox_redis.RedisConfig{Addr: "127.0.0.1:6379"},
		},
	}
}

// Apply 把已有键名补进 SDK 配置。mysql_cfg 只在没有 storage_mysql 时补上。
func Apply(cfg *StorageConfig, legacy *LegacyMysqlConfig) (*StorageConfig, error) {
	if cfg == nil {
		cfg = &StorageConfig{}
	}
	if cfg.Mysql != nil {
		if cfg.Mysql.DBHost == "" {
			cfg.Mysql.DBHost = cfg.Mysql.DBAddress
		}
	}
	if cfg.Sqlite != nil && cfg.Sqlite.DBFilePath == "" {
		cfg.Sqlite.DBFilePath = cfg.Sqlite.Path
	}
	if cfg.PostgreSQL != nil {
		if cfg.PostgreSQL.DBHost == "" {
			cfg.PostgreSQL.DBHost = cfg.PostgreSQL.DBAddress
		}
		if cfg.PostgreSQL.DBSSLMode == "" {
			cfg.PostgreSQL.DBSSLMode = cfg.PostgreSQL.SSLMode
		}
	}
	if cfg.Redis != nil {
		if cfg.Redis.Pwd == "" {
			cfg.Redis.Pwd = cfg.Redis.Password
		}
		if cfg.Redis.DBIndex != nil {
			cfg.Redis.SocketBuck = *cfg.Redis.DBIndex
		}
	}
	if legacy != nil && strings.TrimSpace(legacy.DBName) != "" && cfg.Mysql == nil {
		timeZone := gtbox_orm_config.GTORMTimeZone(legacy.DBTimeZone)
		if timeZone != gtbox_orm_config.GTORMTimeZoneUTC && timeZone != gtbox_orm_config.GTORMTimeZoneShangHai {
			return nil, errors.New("mysql_cfg.dbtimezone must be UTC/0 or Asia/Shanghai/1")
		}
		cfg.Mysql = &StorageMysqlConfig{
			Enabled: true,
			GTORMMysqlConfig: gtbox_orm_mysql.GTORMMysqlConfig{
				DBName:     legacy.DBName,
				DBUser:     legacy.DBUser,
				DBPwd:      legacy.DBPwd,
				DBHost:     legacy.DBAddress,
				DBPort:     int(legacy.DBPort),
				DBTimeZone: timeZone,
			},
		}
	}
	return cfg, StorageConfigValidate(cfg)
}

// StorageConfigValidate 校验主库必须已启用；多个关系库时必须指定唯一主库。
func StorageConfigValidate(cfg *StorageConfig) error {
	if cfg == nil {
		return errors.New("storage_cfg is not configured")
	}
	enabled := map[string]bool{
		"mysql":      cfg.Mysql != nil && cfg.Mysql.Enabled,
		"sqlite":     cfg.Sqlite != nil && cfg.Sqlite.Enabled,
		"postgresql": cfg.PostgreSQL != nil && cfg.PostgreSQL.Enabled,
	}
	if cfg.Primary != "" && !enabled[cfg.Primary] {
		return errors.New("storage_cfg.primary must name an enabled relational database: mysql, sqlite or postgresql")
	}
	count := 0
	for _, active := range enabled {
		if active {
			count++
		}
	}
	if count > 1 && cfg.Primary == "" {
		return errors.New("storage_cfg.primary is required when multiple relational databases are enabled")
	}
	return nil
}

// 让 goccy 识别 legacyPort 的 UnmarshalYAML。
var _ yaml.InterfaceUnmarshaler = (*legacyPort)(nil)
