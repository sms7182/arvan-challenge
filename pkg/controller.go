package pkg

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ypopivniak/queue"
)

type Controller struct {
	CacheService CacheService
	Queue        *queue.ListQueue
}

const MonthKey = "Month:"

func (cr Controller) SetRoutes(e *gin.Engine) {
	e.POST("/data/processor", cr.dataProcessor)
}

func (cr Controller) dataProcessor(c *gin.Context) {
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

	quta, err := cr.CacheService.Get(ctx, MonthKey+dataRequest.UserId)
	if err != nil {
		usage := &UsageQuta{
			monthQuta:  1,
			minuteQuta: 1,
		}
		ujson, err := json.Marshal(usage)
		if err != nil {
			c.JSON(http.StatusBadGateway, "internal error for quta")
			return
		}

		cr.CacheService.Set(ctx, MonthKey+dataRequest.UserId, string(ujson), remainMonthTime(currentTime))
	} else {
		validMonthQuta, err := cr.CacheService.Get(ctx, "UserQuta:"+dataRequest.UserId)
		if err != nil {
			panic("user not found")
		}

		var validQuta UsageQuta
		err = json.Unmarshal([]byte(validMonthQuta), &validQuta)
		if err != nil {
			panic(err.Error())
		}
		var userUsageQuta UsageQuta
		err = json.Unmarshal([]byte(quta), &userUsageQuta)
		if err != nil {
			panic(err.Error())
		}
		var differ = currentTime.Sub(userUsageQuta.lastUsageTime)

		if (differ.Minutes() == 0 && validQuta.minuteQuta < userUsageQuta.minuteQuta) || (validQuta.monthQuta < userUsageQuta.monthQuta &&
			userUsageQuta.lastUsageTime.Add(time.Hour*24).After(currentTime)) {
			c.JSON(http.StatusPaymentRequired, "Quta consumed")
			return
		}

	}

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
