package service

import (
	"context"
	"encoding/json"
	"net/http"

	pulse "pulse.dev"
	"pulse.dev/middleware"
)

//pulse:service
type Service struct{}

type ctxKey struct{}

//pulse:middleware target=all
func (s *Service) InjectContext(req middleware.Request, next middleware.Next) middleware.Response {
	ctx := context.WithValue(req.Context(), ctxKey{}, "svc")
	return next(req.WithContext(ctx))
}

type Response struct {
	Message string `json:"message"`
}

//pulse:api public tag:ctx tag:global
func (s *Service) Context(ctx context.Context) (*Response, error) {
	value, _ := ctx.Value(ctxKey{}).(string)
	return &Response{Message: value}, nil
}

//pulse:api private tag:rewrite
func (s *Service) Private(ctx context.Context) (*Response, error) {
	return &Response{Message: "handler"}, nil
}

//pulse:api public
func (s *Service) CallPrivate(ctx context.Context) (*Response, error) {
	return s.Private(ctx)
}

//pulse:api public tag:error
func (s *Service) Error(ctx context.Context) error {
	return nil
}

//pulse:api public raw path=/raw/:id tag:raw
func (s *Service) Raw(w http.ResponseWriter, req *http.Request) {
	_ = json.NewEncoder(w).Encode(map[string]string{
		"id": pulse.CurrentRequest().PathParams.Get("id"),
	})
}
