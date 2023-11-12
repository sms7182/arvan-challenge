package pkg

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/ypopivniak/queue"
)

type Controller struct {
	Rdb       *redis.Client
	Queue     *queue.ListQueue
	UserQuota map[string]UserQuota
}

func (cr Controller) SetRoutes(e *gin.Engine) {
	e.POST("/data/processor", rateLimiter(cr), cr.dataProcessor)
	e.POST("/quta/init", cr.initQuta)
}

//middleware for check quota : month and minute here check
func rateLimiter(cr Controller) gin.HandlerFunc {
	return func(ctx *gin.Context) {

		var userRequest DataProcessorDto
		reqBody, err := ioutil.ReadAll(ctx.Request.Body)
		if err != nil {
			ctx.JSON(http.StatusBadRequest, err)
			return
		}
		defer ctx.Request.Body.Close()
		_ = json.Unmarshal(reqBody, &userRequest)

		lockKey := fmt.Sprintf(LockKey, userRequest.UserId)
		lock, err := cr.Rdb.Get(ctx, lockKey).Result()
		if err == nil {
			ctx.JSON(http.StatusLocked, lock)
			return
		}

		duplicateKey := fmt.Sprintf(DuplicateKey, userRequest.Id)
		duplicateCnt, err := cr.Rdb.Incr(ctx, duplicateKey).Result()
		if err != nil {
			ctx.JSON(http.StatusBadGateway, err)
			return
		}
		if duplicateCnt > 1 {
			ctx.JSON(http.StatusSeeOther, duplicateCnt)
			return
		}
		go cr.Rdb.Expire(ctx, duplicateKey, time.Hour*24*7)
		userQuota, exist := cr.UserQuota[userRequest.UserId]
		if !exist {
			quta, err := cr.Rdb.Get(ctx, fmt.Sprintf(UserQuotaKey, userRequest.UserId)).Result()
			if err != nil && !errors.Is(err, redis.Nil) {
				log.Printf("redis is not nil for get with key")
				ctx.JSON(http.StatusBadGateway, err)
				return
			} else if errors.Is(err, redis.Nil) {
				log.Printf("redis is nil")
				ctx.JSON(http.StatusNotFound, err)
				return
			}

			err = json.Unmarshal([]byte(quta), &userQuota)
			if err != nil {
				log.Printf("unmarshal has error")
				ctx.JSON(http.StatusBadGateway, err)
				return
			}
			cr.UserQuota[userRequest.UserId] = userQuota
		}

		stateKey := fmt.Sprintf(MinuteUserStateKey, userRequest.UserId)
		countCounterOfMinutes, err := cr.Rdb.Incr(ctx, stateKey).Result()

		if err != nil && err == redis.Nil {
			log.Printf("incr of redis has error")
			ctx.AbortWithError(http.StatusBadGateway, err)

		} else {
			currentTime := time.Now()
			if countCounterOfMinutes > int64(userQuota.MinuteQuta) {

				nextMinute := currentTime.Add(time.Minute)

				cr.Rdb.Expire(ctx, stateKey, time.Duration(nextMinute.Sub(currentTime).Seconds()))

				ctx.AbortWithStatus(http.StatusTooManyRequests)
			} else {
				stateKey = fmt.Sprintf(MonthUserStateKey, userQuota.UserId)
				countCounterOfMonth, err := cr.Rdb.Incr(ctx, stateKey).Result()
				if err != nil && err == redis.Nil {

					ctx.AbortWithError(http.StatusBadGateway, err)
				} else {
					if countCounterOfMonth > int64(userQuota.MonthQuta) {

						go cr.Rdb.Expire(ctx, stateKey, remainMonthTime(currentTime))
						go cr.Rdb.Set(ctx, lockKey, userQuota.UserId, remainMonthTime(currentTime))
						ctx.AbortWithStatus(http.StatusTooManyRequests)

					} else {
						ctx.Next()

					}

				}

			}
		}

	}
}

//initalize for quota
func (cr Controller) initQuta(c *gin.Context) {

	currentTime := time.Now().UTC()
	var userQuta UserQuota
	reqBody, err := ioutil.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, err)
		return
	}
	defer c.Request.Body.Close()
	_ = json.Unmarshal(reqBody, &userQuta)
	if userQuta.MinuteQuta <= 0 || userQuta.MonthQuta <= 0 {
		c.JSON(http.StatusNotAcceptable, userQuta)
		return
	}
	setR, err := cr.Rdb.Set(c, fmt.Sprintf(UserQuotaKey, userQuta.UserId), reqBody, remainMonthTime(currentTime)).Result()
	cr.UserQuota[userQuta.UserId] = userQuta
	if err != nil && !errors.Is(err, redis.Nil) {
		fmt.Printf("redis hs error")
		c.JSON(http.StatusBadGateway, err.Error())
		return
	}

	c.JSON(http.StatusOK, setR)

}

// main request api
func (cr Controller) process(c *gin.Context) {
	currentTime := time.Now().UTC()
	var dataRequest DataProcessorDto
	ctx := context.Background()
	reqBody, err := ioutil.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, err)
		return
	}
	defer c.Request.Body.Close()
	_ = json.Unmarshal(reqBody, &dataRequest)

	cr.Queue.Enqueue(ctx, DataQueue, DataProcessorModel{
		Id:           dataRequest.Id,
		UserId:       dataRequest.UserId,
		ReceivedTime: currentTime,
	})

}
func (cr Controller) dataProcessor(c *gin.Context) {

	go cr.process(c)

}
func remainMonthTime(current time.Time) (remain time.Duration) {
	currentTimestamp := current

	currentYear, currentMonth, _ := currentTimestamp.Date()
	currentLocation := currentTimestamp.Location()
	lastOfMonth := time.Date(currentYear, currentMonth+1, 0, 23, 59, 59, 999999999, currentLocation)
	currentDate := time.Now().UTC()
	differ := lastOfMonth.Sub(currentDate)
	return differ
}
