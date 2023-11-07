package pkg

import "time"

type DataProcessorDto struct {
	Id     string `json:"id"`
	UserId string `json:"userId"`
}

type UsageQuta struct {
	monthQuta     int
	minuteQuta    int
	lastUsageTime time.Time
}
