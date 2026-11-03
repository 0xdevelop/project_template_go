package storage

import (
	"errors"

	"github.com/0xdevelop/project_template_go/config"
	"github.com/0xdevelop/project_template_go/storage/storage_config"
	"github.com/george012/gtbox/gtbox_redis"
)

func openRedis(cfg *storage_config.StorageRedisConfig) error {
	if cfg == nil {
		return errors.New("storage_redis is not configured")
	}
	opened, err := gtbox_redis.New(cfg.RedisConfig, cfg.Prefix)
	if err != nil {
		return err
	}
	globalRedis = opened
	return nil
}

// Redis 返回已启用的 Redis 句柄。读写用 Set、HSet。
func Redis() (*gtbox_redis.GTRedis, error) {
	storageMutex.RLock()
	defer storageMutex.RUnlock()
	if globalRedis == nil {
		return nil, errors.New("storage_redis is not enabled")
	}
	return globalRedis, nil
}

// RedisDatabase 返回当作库使用的 Redis。database 未打开时拒绝。
func RedisDatabase() (*gtbox_redis.GTRedis, error) {
	storageMutex.RLock()
	defer storageMutex.RUnlock()
	cfg := config.GlobalConfig
	if cfg == nil || cfg.StorageCfg == nil || cfg.StorageCfg.Redis == nil || !cfg.StorageCfg.Redis.Enabled || !cfg.StorageCfg.Redis.Database {
		return nil, errors.New("storage_redis.database is not enabled")
	}
	if globalRedis == nil {
		return nil, errors.New("storage_redis is not enabled")
	}
	return globalRedis, nil
}
