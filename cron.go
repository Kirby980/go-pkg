package pkg

import (
	"github.com/robfig/cron/v3"
)

func Timer() *cron.Cron {
	//每秒自动执行，需要加cron.WithSeconds()参数
	c := cron.New(cron.WithSeconds())
	c.Start()
	return c
}
