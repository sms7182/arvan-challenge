package pkg

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/go-redis/redis/v8"
	"golang.org/x/time/rate"
)

type DistributedUserLimiter struct {
	RedisClient *redis.Client
	Policies    map[string]*rate.Limiter
}

func NewDistributedUserLimiter(redisAddr string) (*DistributedUserLimiter, error) {
	client := redis.NewClient(&redis.Options{
		Addr: redisAddr,
		DB:   0,
	})

	return &DistributedUserLimiter{
		RedisClient: client,
		Policies:    make(map[string]*rate.Limiter),
	}, nil
}

func (d *DistributedUserLimiter) SetUserPolicy(userID string, limit rate.Limit) {
	d.Policies[userID] = rate.NewLimiter(limit, 1)
	//limit := rate.Limit(float64(requestsPerMonth) / float64(30*24*60*60)) // 30 days, 24 hours, 60 minutes, 60 seconds

	// Create a new rate limiter with the calculated rate limit

}

func (d *DistributedUserLimiter) Allow(ctx context.Context, userID string) bool {
	limiter, exists := d.Policies[userID]
	if !exists {
		log.Printf("Unknown user: %s", userID)
		return false
	}

	// Use Redis to store rate limiter state
	stateKey := fmt.Sprintf("ratelimit:%s", userID)
	cmd := d.RedisClient.Get(ctx, stateKey)
	if cmd.Err() == redis.Nil {
		// If the key doesn't exist, create it with an initial state
		cmd := d.RedisClient.Set(ctx, stateKey, limiter.AllowN(time.Now(), 1), time.Duration((limiter.Burst())))
		if cmd.Err() != nil {
			log.Printf("Failed to set initial state in Redis: %v", cmd.Err())
			return false
		}
	} else if cmd.Err() != nil {
		log.Printf("Error reading state from Redis: %v", cmd.Err())
		return false
	}

	// Get the current state from Redis
	cmd = d.RedisClient.Get(ctx, stateKey)
	if cmd.Err() != nil {
		log.Printf("Error reading state from Redis: %v", cmd.Err())
		return false
	}

	// Use the rate limiter to check if the request is allowed
	return limiter.AllowN(time.Now(), 1)
}
