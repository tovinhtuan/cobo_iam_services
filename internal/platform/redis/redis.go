package redis

import (
	"context"
	"time"

	"github.com/cobo/cobo_iam_services/internal/platform/config"
	goredis "github.com/redis/go-redis/v9"
)

// Open dials Redis using cfg. Returns (nil, nil) when REDIS_ADDR is empty.
// Ping uses a short timeout; on failure returns (nil, err) so callers can fall back.
func Open(ctx context.Context, cfg config.Config) (*goredis.Client, error) {
	if cfg.RedisAddr == "" {
		return nil, nil
	}
	rdb := goredis.NewClient(&goredis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})
	pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if err := rdb.Ping(pingCtx).Err(); err != nil {
		_ = rdb.Close()
		return nil, err
	}
	return rdb, nil
}
