package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"otp-devops/backend/internal/model"

	"github.com/redis/go-redis/v9"
)

type RedisRepo struct {
	client *redis.Client
}

func NewRedisRepo(addr, password string, db int) (*RedisRepo, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to redis: %w", err)
	}

	return &RedisRepo{client: client}, nil
}

func (r *RedisRepo) otpKey(identifier string) string {
	return fmt.Sprintf("otp:%s", identifier)
}

func (r *RedisRepo) rateLimitKey(identifier string) string {
	return fmt.Sprintf("ratelimit:%s", identifier)
}

// SaveOTP stores an OTP record in Redis with a TTL.
func (r *RedisRepo) SaveOTP(ctx context.Context, identifier string, record model.OTPRecord, ttl time.Duration) error {
	data, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("failed to marshal OTP record: %w", err)
	}

	return r.client.Set(ctx, r.otpKey(identifier), data, ttl).Err()
}

// GetOTP retrieves an OTP record from Redis.
func (r *RedisRepo) GetOTP(ctx context.Context, identifier string) (*model.OTPRecord, error) {
	data, err := r.client.Get(ctx, r.otpKey(identifier)).Result()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get OTP record: %w", err)
	}

	var record model.OTPRecord
	if err := json.Unmarshal([]byte(data), &record); err != nil {
		return nil, fmt.Errorf("failed to unmarshal OTP record: %w", err)
	}

	return &record, nil
}

// DeleteOTP removes an OTP record from Redis.
func (r *RedisRepo) DeleteOTP(ctx context.Context, identifier string) error {
	return r.client.Del(ctx, r.otpKey(identifier)).Err()
}

// UpdateOTP overwrites an OTP record keeping the existing TTL.
func (r *RedisRepo) UpdateOTP(ctx context.Context, identifier string, record model.OTPRecord) error {
	key := r.otpKey(identifier)

	// Get remaining TTL so we don't reset it
	ttl, err := r.client.TTL(ctx, key).Result()
	if err != nil {
		return fmt.Errorf("failed to get TTL: %w", err)
	}
	if ttl <= 0 {
		// Key already expired or doesn't exist
		return fmt.Errorf("OTP record not found or expired")
	}

	data, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("failed to marshal OTP record: %w", err)
	}

	return r.client.Set(ctx, key, data, ttl).Err()
}

// SetRateLimit sets a rate-limit key for an identifier with a TTL.
func (r *RedisRepo) SetRateLimit(ctx context.Context, identifier string, ttl time.Duration) error {
	return r.client.Set(ctx, r.rateLimitKey(identifier), "1", ttl).Err()
}

// IsRateLimited checks if an identifier is currently rate-limited.
func (r *RedisRepo) IsRateLimited(ctx context.Context, identifier string) (bool, error) {
	exists, err := r.client.Exists(ctx, r.rateLimitKey(identifier)).Result()
	if err != nil {
		return false, fmt.Errorf("failed to check rate limit: %w", err)
	}
	return exists > 0, nil
}

// Close closes the Redis connection.
func (r *RedisRepo) Close() error {
	return r.client.Close()
}
