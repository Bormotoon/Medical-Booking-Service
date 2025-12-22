package repository

import (
	"context"
	"fmt"

	"bronivik/internal/config"
	"github.com/redis/go-redis/v9"
)

// NewRedisClient создает новый клиент Redis на основе конфигурации
func NewRedisClient(cfg config.RedisConfig) *redis.Client {
	options := &redis.Options{
		Addr:     cfg.Address,
		Password: cfg.Password,
		DB:       cfg.DB,
		PoolSize: cfg.PoolSize,
		// MinIdleConns: cfg.MinIdleConns,
		// DialTimeout:  time.Duration(cfg.DialTimeout) * time.Second,
		// ReadTimeout:  time.Duration(cfg.ReadTimeout) * time.Second,
		// WriteTimeout: time.Duration(cfg.WriteTimeout) * time.Second,
		// PoolTimeout:  time.Duration(cfg.PoolTimeout) * time.Second,
		// IdleTimeout:  time.Duration(cfg.IdleTimeout) * time.Minute,
	}

	client := redis.NewClient(options)

	return client
}

// Ping проверяет соединение с Redis
func Ping(ctx context.Context, client *redis.Client) error {
	_, err := client.Ping(ctx).Result()
	if err != nil {
		return fmt.Errorf("failed to ping Redis: %w", err)
	}
	return nil
}

// Close закрывает соединение с Redis
func Close(client *redis.Client) error {
	if client != nil {
		return client.Close()
	}
	return nil
}
