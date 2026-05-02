package mw

import (
	service "example.com/middlewareapp/service"

	"github.com/pbrazdil/onlava/errs"
	"github.com/pbrazdil/onlava/middleware"
)

//onlava:middleware target=tag:rewrite
func Rewrite(req middleware.Request, next middleware.Next) middleware.Response {
	resp := next(req)
	payload := resp.Payload.(*service.Response)
	payload.Message = "middleware:" + payload.Message
	return resp
}

//onlava:middleware target=tag:error
func Error(req middleware.Request, next middleware.Next) middleware.Response {
	return middleware.Response{
		Err: errs.B().Code(errs.Internal).Msg("middleware error").Err(),
	}
}

//onlava:middleware target=tag:raw
func RawHeader(req middleware.Request, next middleware.Next) middleware.Response {
	resp := next(req)
	resp.Header().Set("X-Raw-Middleware", "true")
	return resp
}
