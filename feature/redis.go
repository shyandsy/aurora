package feature

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/go-redis/redis/v8"

	"github.com/shyandsy/aurora/config"
	"github.com/shyandsy/aurora/contracts"
)

type RedisService interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error
	Delete(ctx context.Context, keys ...string) (int64, error)
	Exists(ctx context.Context, key string) (bool, error)
	Incr(ctx context.Context, key string) (int64, error)
	SetNX(ctx context.Context, key string, value interface{}, expiration time.Duration) (bool, error)
	// Hash operations
	HSet(ctx context.Context, key, field string, value interface{}) error
	HGet(ctx context.Context, key, field string) (string, error)
	HDel(ctx context.Context, key string, fields ...string) (int64, error)
	HGetAll(ctx context.Context, key string) (map[string]string, error)
	HExists(ctx context.Context, key, field string) (bool, error)
	HKeys(ctx context.Context, key string) ([]string, error)
	Expire(ctx context.Context, key string, expiration time.Duration) error
}

type redisFeature struct {
	client *redis.Client
	config *config.RedisConfig
}

func NewRedisFeature() contracts.Features {
	cfg := &config.RedisConfig{}
	if err := config.ResolveConfig(cfg); err != nil {
		log.Fatalf("Failed to load redis config: %v", err)
	}
	return &redisFeature{config: cfg}
}

func (f *redisFeature) Name() string {
	return "redis"
}

func (f *redisFeature) Setup(app contracts.App) error {
	if err := f.config.Validate(); err != nil {
		return fmt.Errorf("redis configuration validation failed: %w", err)
	}

	var err error
	f.client, err = f.provideRedis()
	if err != nil {
		return err
	}

	redisSvc := &redisService{client: f.client}
	app.ProvideAs(redisSvc, (*RedisService)(nil))
	return nil
}

func (f *redisFeature) Close() error {
	if f.client != nil {
		return f.client.Close()
	}
	return nil
}

func (f *redisFeature) provideRedis() (*redis.Client, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     f.config.Addr,
		Password: f.config.Password,
		DB:       f.config.DB,
	})

	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, err
	}

	return client, nil
}

type redisService struct {
	client *redis.Client
}

func (r *redisService) Get(ctx context.Context, key string) (string, error) {
	val, err := r.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return "", nil
	}
	return val, err
}

func (r *redisService) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	return r.client.Set(ctx, key, value, expiration).Err()
}

func (r *redisService) Delete(ctx context.Context, keys ...string) (int64, error) {
	cmd := r.client.Del(ctx, keys...)
	return cmd.Result()
}

func (r *redisService) Exists(ctx context.Context, key string) (bool, error) {
	n, err := r.client.Exists(ctx, key).Result()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

func (r *redisService) Incr(ctx context.Context, key string) (int64, error) {
	return r.client.Incr(ctx, key).Result()
}

func (r *redisService) SetNX(ctx context.Context, key string, value interface{}, expiration time.Duration) (bool, error) {
	return r.client.SetNX(ctx, key, value, expiration).Result()
}

// HSet sets a field in a hash stored at key
func (r *redisService) HSet(ctx context.Context, key, field string, value interface{}) error {
	return r.client.HSet(ctx, key, field, value).Err()
}

// HGet gets a field value from a hash stored at key
func (r *redisService) HGet(ctx context.Context, key, field string) (string, error) {
	val, err := r.client.HGet(ctx, key, field).Result()
	if err == redis.Nil {
		return "", nil
	}
	return val, err
}

// HDel deletes one or more fields from a hash stored at key
func (r *redisService) HDel(ctx context.Context, key string, fields ...string) (int64, error) {
	return r.client.HDel(ctx, key, fields...).Result()
}

// HGetAll gets all fields and values from a hash stored at key
func (r *redisService) HGetAll(ctx context.Context, key string) (map[string]string, error) {
	return r.client.HGetAll(ctx, key).Result()
}

// HExists checks if a field exists in a hash stored at key
func (r *redisService) HExists(ctx context.Context, key, field string) (bool, error) {
	return r.client.HExists(ctx, key, field).Result()
}

// HKeys gets all field names from a hash stored at key
func (r *redisService) HKeys(ctx context.Context, key string) ([]string, error) {
	return r.client.HKeys(ctx, key).Result()
}

// Expire sets an expiration time on a key
func (r *redisService) Expire(ctx context.Context, key string, expiration time.Duration) error {
	return r.client.Expire(ctx, key, expiration).Err()
}
