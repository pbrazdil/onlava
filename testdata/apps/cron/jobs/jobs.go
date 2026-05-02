package jobs

import (
	"example.com/cronapp/service"
	"github.com/pbrazdil/onlava/cron"
)

var _ = cron.NewJob("onlava-tick", cron.JobConfig{
	Title:    "Onlava Tick",
	Every:    1,
	Endpoint: service.Run,
})
