package store

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisStore struct {
	client *redis.Client
	pubsub *redis.PubSub
}

// NewRedisStore creates a new Redis store for caching and pub/sub
func NewRedisStore(addr, password string, db int) (*RedisStore, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
		PoolSize: 20,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis ping: %w", err)
	}

	return &RedisStore{client: client}, nil
}

// Cache helpers
func (r *RedisStore) SetJSON(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return r.client.Set(ctx, key, data, ttl).Err()
}

func (r *RedisStore) GetJSON(ctx context.Context, key string, dest interface{}) error {
	data, err := r.client.Get(ctx, key).Bytes()
	if err != nil {
		return err
	}
	return json.Unmarshal(data, dest)
}

func (r *RedisStore) Delete(ctx context.Context, keys ...string) error {
	return r.client.Del(ctx, keys...).Err()
}

// Cache invalidation helpers
func (r *RedisStore) InvalidateProject(ctx context.Context, projectID string) {
	r.client.Del(ctx, fmt.Sprintf("project:%s", projectID))
	r.client.Del(ctx, fmt.Sprintf("project:%s:flags", projectID))
	r.client.Del(ctx, fmt.Sprintf("project:%s:users", projectID))
	r.client.Del(ctx, fmt.Sprintf("project:%s:audit", projectID))
}

func (r *RedisStore) InvalidateFlag(ctx context.Context, projectID, flagID string) {
	r.client.Del(ctx, fmt.Sprintf("flag:%s", flagID))
	r.client.Del(ctx, fmt.Sprintf("project:%s:flags", projectID))
}

func (r *RedisStore) InvalidateUser(ctx context.Context, userID string) {
	r.client.Del(ctx, fmt.Sprintf("user:%s", userID))
}

// Pub/Sub for cross-instance broadcast
func (r *RedisStore) Subscribe(ctx context.Context, channel string) *redis.PubSub {
	return r.client.Subscribe(ctx, channel)
}

func (r *RedisStore) Publish(ctx context.Context, channel string, message interface{}) error {
	data, err := json.Marshal(message)
	if err != nil {
		return err
	}
	return r.client.Publish(ctx, channel, data).Err()
}

// Presence tracking - track connected clients per project
func (r *RedisStore) AddClient(ctx context.Context, projectID, clientID string) error {
	key := fmt.Sprintf("project:%s:clients", projectID)
	return r.client.SAdd(ctx, key, clientID).Err()
}

func (r *RedisStore) RemoveClient(ctx context.Context, projectID, clientID string) error {
	key := fmt.Sprintf("project:%s:clients", projectID)
	return r.client.SRem(ctx, key, clientID).Err()
}

func (r *RedisStore) GetClients(ctx context.Context, projectID string) ([]string, error) {
	key := fmt.Sprintf("project:%s:clients", projectID)
	return r.client.SMembers(ctx, key).Result()
}

// Distributed lock for critical operations
func (r *RedisStore) Lock(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	return r.client.SetNX(ctx, key, "1", ttl).Result()
}

func (r *RedisStore) Unlock(ctx context.Context, key string) error {
	return r.client.Del(ctx, key).Err()
}

func (r *RedisStore) Close() error {
	return r.client.Close()
}
