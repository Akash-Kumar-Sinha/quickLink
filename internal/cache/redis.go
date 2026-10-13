package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type options struct {
	defaultTTL  time.Duration
	dialTimeout time.Duration
}

type Option func(*options)

func WithDefaultTTL(ttl time.Duration) Option {
	return func(o *options) {
		if ttl > 0 {
			o.defaultTTL = ttl
		}
	}
}

func WithDialTimeout(t time.Duration) Option {
	return func(o *options) {
		if t > 0 {
			o.dialTimeout = t
		}
	}
}

type Client struct {
	rdb        *redis.Client
	defaultTTL time.Duration
}

func NewRedis(redisURL string, opts ...Option) (*Client, error) {
	o := &options{
		defaultTTL:  24 * time.Hour,
		dialTimeout: 5 * time.Second,
	}

	for _, opt := range opts {
		opt(o)
	}

	redisOpt, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse redis url: %w", err)
	}
	redisOpt.DialTimeout = o.dialTimeout

	rdb := redis.NewClient(redisOpt)

	ctx, cancel := context.WithTimeout(context.Background(), o.dialTimeout)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to ping redis: %w", err)
	}

	return &Client{
		rdb:        rdb,
		defaultTTL: o.defaultTTL,
	}, nil
}

func (c *Client) Get(ctx context.Context, code string) (string, error) {
	return c.rdb.Get(ctx, "url:"+code).Result()
}

func (c *Client) Set(ctx context.Context, code string, originalURL string, ttl time.Duration) error {
	effectiveTTL := ttl
	if effectiveTTL <= 0 {
		effectiveTTL = c.defaultTTL
	}
	return c.rdb.Set(ctx, "url:"+code, originalURL, effectiveTTL).Err()
}

func (c *Client) Del(ctx context.Context, code string) error {
	return c.rdb.Del(ctx, "url:"+code).Err()
}

func (c *Client) DefaultTTL() time.Duration {
	return c.defaultTTL
}

func (c *Client) Close() error {
	return c.rdb.Close()
}
