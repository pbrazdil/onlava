package service

import (
	"context"
	"encoding/json"
	"net/http"

	onlava "github.com/pbrazdil/onlava"
	"github.com/pbrazdil/onlava/middleware"
)

//onlava:service
type Service struct{}

type ctxKey struct{}

//onlava:middleware target=all
func (s *Service) InjectContext(req middleware.Request, next middleware.Next) middleware.Response {
	ctx := context.WithValue(req.Context(), ctxKey{}, "svc")
	return next(req.WithContext(ctx))
}

type Response struct {
	Message string `json:"message"`
}

//onlava:api public tag:ctx tag:global
func (s *Service) Context(ctx context.Context) (*Response, error) {
	value, _ := ctx.Value(ctxKey{}).(string)
	return &Response{Message: value}, nil
}

//onlava:api private tag:rewrite
func (s *Service) Private(ctx context.Context) (*Response, error) {
	return &Response{Message: "handler"}, nil
}

//onlava:api public
func (s *Service) CallPrivate(ctx context.Context) (*Response, error) {
	return s.Private(ctx)
}

//onlava:api public tag:error
func (s *Service) Error(ctx context.Context) error {
	return nil
}

//onlava:api public raw path=/raw/:id tag:raw
func (s *Service) Raw(w http.ResponseWriter, req *http.Request) {
	_ = json.NewEncoder(w).Encode(map[string]string{
		"id": onlava.CurrentRequest().PathParams.Get("id"),
	})
}
