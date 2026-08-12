package feature

import (
	"context"
	"fmt"
	"log"
	"sync"
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
	// Distributed lock operations
	// WithLock executes a function with a distributed lock
	// It automatically acquires the lock, executes the function, and releases the lock
	// The lock is automatically refreshed (extended) while the function is executing
	// If the lock cannot be acquired, the function is not executed and returns ErrLockNotAcquired
	// If the context is cancelled, the lock is automatically released
	// The lock will automatically expire after ttl duration if the process crashes
	WithLock(ctx context.Context, key string, value string, ttl time.Duration, fn func() error) error

	// Pub/Sub operations
	// Publish sends a message to a channel (fire-and-forget; no error if there are no subscribers).
	Publish(ctx context.Context, channel string, message string) error
	// Subscribe subscribes to a channel and returns a receive-only channel of message payloads
	// plus a close function. The internal read loop auto-reconnects. The returned channel is closed
	// when the close function is called or ctx is cancelled; callers MUST call close (or cancel ctx)
	// when done, to release the underlying connection.
	Subscribe(ctx context.Context, channel string) (<-chan string, func() error, error)
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

// ErrLockNotAcquired is returned when WithLock cannot acquire the lock
var ErrLockNotAcquired = fmt.Errorf("lock not acquired")

// WithLock executes a function with a distributed lock
// It automatically acquires the lock, executes the function, and releases the lock
// The lock is automatically refreshed (extended) while the function is executing
// If the lock cannot be acquired, the function is not executed and returns ErrLockNotAcquired
// If the context is cancelled, the lock is automatically released
// The lock will automatically expire after ttl duration if the process crashes
func (r *redisService) WithLock(ctx context.Context, key string, value string, ttl time.Duration, fn func() error) error {
	// Try to acquire lock using Redis SetNX
	acquired, err := r.client.SetNX(ctx, key, value, ttl).Result()
	if err != nil {
		return fmt.Errorf("failed to acquire lock: %w", err)
	}
	if !acquired {
		return ErrLockNotAcquired
	}

	// Create a context that will be cancelled when we need to stop refreshing
	refreshCtx, cancelRefresh := context.WithCancel(ctx)
	defer cancelRefresh()

	// Start a goroutine to refresh the lock periodically
	// This ensures the lock doesn't expire if the task takes longer than ttl
	refreshDone := make(chan struct{})
	go func() {
		defer close(refreshDone)
		refreshInterval := ttl / 2 // Refresh at half the TTL to ensure we refresh before expiration
		if refreshInterval < time.Second {
			refreshInterval = time.Second // Minimum 1 second
		}

		ticker := time.NewTicker(refreshInterval)
		defer ticker.Stop()

		for {
			select {
			case <-refreshCtx.Done():
				return
			case <-ticker.C:
				// Refresh the lock by extending its TTL
				// Only refresh if the lock value matches (we still own it)
				currentValue, err := r.client.Get(refreshCtx, key).Result()
				if err != nil {
					// Lock may have been released or expired, stop refreshing
					return
				}
				if currentValue == value {
					// We still own the lock, extend it
					if err := r.client.Expire(refreshCtx, key, ttl).Err(); err != nil {
						// Failed to refresh, stop trying
						return
					}
				} else {
					// Lock value changed, we no longer own it, stop refreshing
					return
				}
			}
		}
	}()

	// Ensure lock is released when function completes or context is cancelled
	defer func() {
		// Stop the refresh goroutine
		cancelRefresh()
		<-refreshDone

		// Release the lock by deleting the key
		// Use a short timeout context to avoid blocking
		releaseCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if _, err := r.client.Del(releaseCtx, key).Result(); err != nil {
			// Log error but don't fail - lock will expire automatically
			log.Printf("Warning: failed to release lock %s: %v", key, err)
		}
	}()

	// Execute the function
	// If context is cancelled, the function should handle it
	return fn()
}

// Publish implements RedisService.Publish.
func (r *redisService) Publish(ctx context.Context, channel string, message string) error {
	return r.client.Publish(ctx, channel, message).Err()
}

// Subscribe implements RedisService.Subscribe.
//
// It confirms the subscription is established before returning, then pumps message payloads into
// the returned channel from an auto-reconnecting read loop. The returned channel is closed (and the
// underlying connection released) when the returned close function is called or ctx is cancelled.
func (r *redisService) Subscribe(ctx context.Context, channel string) (<-chan string, func() error, error) {
	sub := r.client.Subscribe(ctx, channel)
	// The first Receive returns the subscription confirmation; a failure here means we never
	// actually subscribed, so surface it instead of returning a channel that never delivers.
	if _, err := sub.Receive(ctx); err != nil {
		_ = sub.Close()
		return nil, nil, fmt.Errorf("subscribe %q failed: %w", channel, err)
	}

	// close is idempotent: both the caller's close func and the goroutine's defer route through it,
	// so the PubSub is closed exactly once regardless of which path fires first.
	var once sync.Once
	var closeErr error
	closeFn := func() error {
		once.Do(func() { closeErr = sub.Close() })
		return closeErr
	}

	out := make(chan string, 16)
	go func() {
		defer close(out)
		defer closeFn() // ctx cancellation / loop exit also releases the connection (no leak)
		ch := sub.Channel()
		for {
			select {
			case msg, ok := <-ch:
				if !ok {
					return // sub.Close() was called → channel closed → stop
				}
				select {
				case out <- msg.Payload:
				case <-ctx.Done():
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	return out, closeFn, nil
}
