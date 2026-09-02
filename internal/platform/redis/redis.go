package redis

import (
	"context"
	"errors"
	"fmt"

	goredis "github.com/redis/go-redis/v9"
)

var ErrDisabled = errors.New("redis is not configured")

type Redis struct {
	client *goredis.Client
}

func Open(url string) (*Redis, error) {
	if url == "" {
		return &Redis{}, nil
	}
	options, err := goredis.ParseURL(url)
	if err != nil {
		return nil, fmt.Errorf("parse REDIS_URL: %w", err)
	}
	return &Redis{client: goredis.NewClient(options)}, nil
}

func (r *Redis) Enabled() bool { return r != nil && r.client != nil }

func (r *Redis) Client() *goredis.Client {
	if r == nil {
		return nil
	}
	return r.client
}

func (r *Redis) Ping(ctx context.Context) error {
	if !r.Enabled() {
		return ErrDisabled
	}
	return r.client.Ping(ctx).Err()
}

func (r *Redis) Close() error {
	if !r.Enabled() {
		return nil
	}
	return r.client.Close()
}
