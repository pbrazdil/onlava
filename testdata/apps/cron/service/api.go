package service

import (
	"context"
	"sync"

	onlava "github.com/pbrazdil/onlava"
)

var (
	cronMu    sync.Mutex
	cronState StatusResponse
)

type StatusResponse struct {
	Count int    `json:"count"`
	Cron  string `json:"cron"`
	Type  string `json:"type"`
	Path  string `json:"path"`
}

//onlava:api private
func Run(ctx context.Context) error {
	req := onlava.CurrentRequest()

	cronMu.Lock()
	defer cronMu.Unlock()
	cronState.Count++
	cronState.Cron = req.CronIdempotencyKey
	cronState.Type = string(req.Type)
	cronState.Path = req.Path
	return nil
}

//onlava:api public path=/cron/status method=GET
func Status(ctx context.Context) (*StatusResponse, error) {
	cronMu.Lock()
	defer cronMu.Unlock()
	resp := cronState
	return &resp, nil
}
