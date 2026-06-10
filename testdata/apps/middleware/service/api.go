package service

import (
	"context"
	"encoding/json"
	"net/http"

	scenery "scenery.sh"
	"scenery.sh/middleware"
)

//scenery:service
type Service struct{}

type ctxKey struct{}

//scenery:middleware target=all
func (s *Service) InjectContext(req middleware.Request, next middleware.Next) middleware.Response {
	ctx := context.WithValue(req.Context(), ctxKey{}, "svc")
	return next(req.WithContext(ctx))
}

type Response struct {
	Message string `json:"message"`
}

//scenery:api public tag:ctx tag:global
func (s *Service) Context(ctx context.Context) (*Response, error) {
	value, _ := ctx.Value(ctxKey{}).(string)
	return &Response{Message: value}, nil
}

//scenery:api private tag:rewrite
func (s *Service) Private(ctx context.Context) (*Response, error) {
	return &Response{Message: "handler"}, nil
}

//scenery:api public
func (s *Service) CallPrivate(ctx context.Context) (*Response, error) {
	return s.Private(ctx)
}

//scenery:api public tag:error
func (s *Service) Error(ctx context.Context) error {
	return nil
}

//scenery:api public raw path=/raw/:id tag:raw
func (s *Service) Raw(w http.ResponseWriter, req *http.Request) {
	_ = json.NewEncoder(w).Encode(map[string]string{
		"id": scenery.CurrentRequest().PathParams.Get("id"),
	})
}
