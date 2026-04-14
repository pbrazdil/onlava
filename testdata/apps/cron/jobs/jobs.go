package jobs

import (
	"example.com/cronapp/service"
	"pulse.dev/cron"
)

var _ = cron.NewJob("pulse-tick", cron.JobConfig{
	Title:    "Pulse Tick",
	Every:    1,
	Endpoint: service.Run,
})
