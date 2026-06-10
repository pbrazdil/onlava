package jobs

import (
	"example.com/cronapp/service"
	"scenery.sh/cron"
)

var _ = cron.NewJob("scenery-tick", cron.JobConfig{
	Title:    "scenery Tick",
	Every:    1,
	Endpoint: service.Run,
})
