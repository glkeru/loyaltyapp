package points

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	redis "github.com/redis/go-redis/v9"
)

type CacheService struct {
	client *redis.Client
}

func NewCacheService() (serv *CacheService, err error) {

	// config
	addr := os.Getenv("POINTS_CACHE_URL")
	if addr == "" {
		return nil, fmt.Errorf("env POINTS_CACHE_URL is not set")
	}
	user := os.Getenv("POINTS_CACHE_USER")
	if addr == "" {
		return nil, fmt.Errorf("env POINTS_CACHE_USER is not set")
	}
	pwd := os.Getenv("POINTS_CACHE_PWD")
	if addr == "" {
		return nil, fmt.Errorf("env POINTS_CACHE_PWD is not set")
	}
	// redis
	db := redis.NewClient(&redis.Options{
		Addr:        addr,
		Password:    pwd,
		Username:    user,
		DB:          0,
		MaxRetries:  5,
		DialTimeout: 10 * time.Second,
	})
	err = db.Ping(context.Background()).Err()
	if err != nil {
		return nil, err
	}

	return &CacheService{db}, nil
}

func (c *CacheService) GetBalance(ctx context.Context, user string) (points float64, err error) {
	val, err := c.client.Get(ctx, user).Result()
	if err == redis.Nil {
		return 0, fmt.Errorf("not found")
	} else if err != nil {
		return 0, err
	}

	points, err = strconv.ParseFloat(val, 64)
	if err != nil {
		return 0, err

	}
	return points, nil
}

func (c *CacheService) SetBalance(ctx context.Context, user string, points float64) (err error) {
	err = c.client.Set(ctx, user, points, 5*time.Minute).Err()
	if err != nil {
		return err
	}
	return nil
}

func (c *CacheService) InvalidateBalance(ctx context.Context, user string) error {
	err := c.client.Del(ctx, user).Err()
	if err != nil {
		return err
	}
	return nil
}
