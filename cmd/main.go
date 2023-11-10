package main

import (
	"log"
	"os"
	"processor-challenge/pkg"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/spf13/viper"
	"github.com/ypopivniak/queue"
	"golang.org/x/time/rate"
)

func main() {
	setUpViper()
	//redisClient := getRedisClient()

	limiter, err := pkg.NewDistributedUserLimiter("localhost:6379")
	if err != nil {
		log.Fatal("Failed to create limiter:", err)
	}

	client := redis.NewClient(&redis.Options{
		Addr: viper.GetString("queueRedis.urls"),
	})
	limiter.SetUserPolicy("1", rate.Limit(2))
	que := queue.NewListQueue(client, &queue.Options{})

	cacheService := pkg.CacheService{
		//Rdb: redisClient,
	}
	controller := pkg.Controller{
		CacheService: cacheService,
		Queue:        que,
		Limiter:      limiter,
	}

	router := gin.New()
	controller.SetRoutes(router)
	router.Run(viper.GetString("serverPort"))
}

func getRedisClient() *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr:     viper.GetString("cacheRedis.urls"),
		Password: viper.GetString("cacheRedis.password"),
		DB:       viper.GetInt("cacheRedis.db"),
	})
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func setUpViper() {
	viper.SetConfigName(getEnv("CONFIG_NAME", "dev-conf"))
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./conf")
	err := viper.ReadInConfig()
	if err != nil {
		log.Fatalf("Fatal error config file: %+v \n", err)
	}
}
