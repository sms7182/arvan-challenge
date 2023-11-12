package pkg

import "time"

type DataProcessorDto struct {
	Id     string `json:"id"`
	UserId string `json:"userId"`
}

type UserQuota struct {
	MonthQuta  int    `json:"monthQuta"`
	MinuteQuta int    `json:"minuteQuta"`
	UserId     string `json:"userId"`
}

type DataProcessorModel struct {
	Id           string
	UserId       string
	ReceivedTime time.Time
}
