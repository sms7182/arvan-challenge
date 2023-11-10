package pkg

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/ypopivniak/queue"
)

type Controller struct {
	CacheService CacheService
	Queue        *queue.ListQueue
	Limiter      *DistributedUserLimiter
}

const DataQueue = "DataQueue"
const UserQutaKey = "UserQuta:%s"
const UserUsageKey = "UserUsage:%s"
const BlockMinKey = "BlockMin:%s"

func (cr Controller) SetRoutes(e *gin.Engine) {
	e.POST("/data/processor", rateLimiter(cr), cr.dataProcessor)
	e.POST("/quta/init", cr.initQuta)
}

func rateLimiter(cr Controller) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		var userQuta UserQuta
		reqBody, err := ioutil.ReadAll(ctx.Request.Body)
		if err != nil {
			ctx.JSON(http.StatusBadRequest, err)
			return
		}
		defer ctx.Request.Body.Close()
		_ = json.Unmarshal(reqBody, &userQuta)

		if cr.Limiter.Allow(ctx, userQuta.UserId) {
			ctx.Next()
		} else {
			ctx.AbortWithStatus(http.StatusTooManyRequests)
		}

	}
}

func (cr Controller) initQuta(c *gin.Context) {
	currentTime := time.Now().UTC()
	ctx := context.Background()
	var userQuta UserQuta
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
	ujson, err := json.Marshal(userQuta)
	if err != nil {
		c.JSON(http.StatusBadGateway, "internal error for quta")
		return
	}
	err = cr.CacheService.Set(ctx, fmt.Sprintf(UserQutaKey, userQuta.UserId), string(ujson), remainMonthTime(currentTime))
	if err != nil {
		c.JSON(http.StatusBadGateway, err.Error())
		return
	}
	c.JSON(http.StatusOK, userQuta)

}
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

	cr.CacheService.Get(ctx, fmt.Sprintf(BlockMinKey, dataRequest.UserId))
	existKey := dataRequest.Id + ":" + dataRequest.UserId
	res, err := cr.CacheService.Get(ctx, existKey)

	if err == nil {
		c.JSON(http.StatusSeeOther, fmt.Sprintf("duplicate data for id:%s", res))
		return
	}
	if !errors.Is(err, redis.Nil) {

		c.JSON(http.StatusSeeOther, fmt.Sprintf("some error in redis:%s", err))
		return
	}

	quta, err := cr.CacheService.Get(ctx, fmt.Sprintf(UserQutaKey, dataRequest.UserId))
	if err != nil && !errors.Is(err, redis.Nil) {
		c.JSON(http.StatusBadGateway, "internal error for quta")
		return

	} else if errors.Is(err, redis.Nil) {
		c.JSON(http.StatusNotFound, fmt.Sprintf("user with %s not found", dataRequest.UserId))
		return
	} else {

		var validQuta UserQuta
		err = json.Unmarshal([]byte(quta), &validQuta)
		if err != nil {
			c.JSON(http.StatusBadGateway, err)
			return
		}
		usageQuta, err := cr.CacheService.Get(ctx, fmt.Sprintf(UserUsageKey, dataRequest.UserId))
		if err != nil {
			c.JSON(http.StatusBadGateway, err)
			return
		}
		var userUsageQuta UsageQuta
		err = json.Unmarshal([]byte(usageQuta), &userUsageQuta)
		if err != nil {
			c.JSON(http.StatusBadGateway, err)
			return

		}
		//	var differ = currentTime.Sub(userUsageQuta.LastUsageTime)

		// if differ.Minutes() == 0 {
		// 	if validQuta.MinuteQuta < userUsageQuta.MinuteQuta {
		// 		dif := time.Duration(60 - currentTime.Second())
		// 		cr.CacheService.Set(ctx, fmt.Sprintf(BlockMinKey, dataRequest.UserId), dataRequest.UserId, time.Second*dif)
		// 		c.JSON(http.StatusPaymentRequired, "Quta consumed")
		// 		return
		// 	} else {
		// 		userUsageQuta.MinuteQuta += 1
		// 	}

		// } else {
		// 	userUsageQuta.MinuteQuta = 1
		// }

		// if validQuta.MonthQuta < userUsageQuta.MonthQuta && userUsageQuta.LastUsageTime.Add(time.Hour*24).After(currentTime) {

		// }

		// userUsageQuta.LastUsageTime = currentTime

		// userUsageQuta.MonthQuta += 1
		// js, err := json.Marshal(userUsageQuta)
		// if err != nil {
		// 	c.JSON(http.StatusBadGateway, err)
		// 	return
		// }
		// //zero
		// cr.CacheService.Set(ctx, fmt.Sprintf(UserUsageKey, dataRequest.UserId), string(js), remainMonthTime(currentTime))

	}
	cr.Queue.Enqueue(ctx, DataQueue, DataProcessorModel{
		Id:           dataRequest.Id,
		UserId:       dataRequest.UserId,
		ReceivedTime: currentTime,
	})
	cr.CacheService.Set(ctx, existKey, dataRequest.Id, time.Hour*24)

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
