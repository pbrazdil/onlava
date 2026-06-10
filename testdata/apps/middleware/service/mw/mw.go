package mw

import (
	service "example.com/middlewareapp/service"

	"scenery.sh/errs"
	"scenery.sh/middleware"
)

//scenery:middleware target=tag:rewrite
func Rewrite(req middleware.Request, next middleware.Next) middleware.Response {
	resp := next(req)
	payload := resp.Payload.(*service.Response)
	payload.Message = "middleware:" + payload.Message
	return resp
}

//scenery:middleware target=tag:error
func Error(req middleware.Request, next middleware.Next) middleware.Response {
	return middleware.Response{
		Err: errs.B().Code(errs.Internal).Msg("middleware error").Err(),
	}
}

//scenery:middleware target=tag:raw
func RawHeader(req middleware.Request, next middleware.Next) middleware.Response {
	resp := next(req)
	resp.Header().Set("X-Raw-Middleware", "true")
	return resp
}
