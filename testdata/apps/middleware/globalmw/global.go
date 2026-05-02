package globalmw

import "github.com/pbrazdil/onlava/middleware"

//onlava:middleware global target=tag:global
func AddHeader(req middleware.Request, next middleware.Next) middleware.Response {
	resp := next(req)
	resp.Header().Set("X-Global-Middleware", "true")
	return resp
}
