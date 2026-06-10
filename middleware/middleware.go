package middleware

import (
	"context"
	"net/http"
	"sync"

	"scenery.sh/runtime/shared"
)

// Signature documents the function signature middleware must implement.
type Signature func(req Request, next Next) Response

// Next invokes the next middleware in the chain or the endpoint handler.
type Next func(Request) Response

// Request describes the request currently being processed by middleware.
type Request struct {
	ctx   context.Context
	cache *reqCache
}

// WithContext returns a copy of the request using ctx.
func (r Request) WithContext(ctx context.Context) Request {
	r.ctx = ctx
	return r
}

// Context reports the request context.
func (r Request) Context() context.Context {
	return r.ctx
}

// Data returns the current request metadata.
func (r Request) Data() *shared.Request {
	if r.cache == nil {
		return &shared.Request{Type: shared.None}
	}
	return r.cache.Get()
}

// Response represents the middleware chain result.
type Response struct {
	Payload    any
	Err        error
	HTTPStatus int
	headers    http.Header
}

// Header returns the response headers that will be written out.
func (r *Response) Header() http.Header {
	if r.headers == nil {
		r.headers = make(http.Header)
	}
	return r.headers
}

// GetHeaders exposes the current header map for runtime use.
func (r *Response) GetHeaders() http.Header {
	return r.headers
}

// NewRequest constructs a middleware request with eager request metadata.
func NewRequest(ctx context.Context, data *shared.Request) Request {
	return NewLazyRequest(ctx, func() *shared.Request { return data })
}

// NewLazyRequest constructs a middleware request with lazily loaded metadata.
func NewLazyRequest(ctx context.Context, fn func() *shared.Request) Request {
	return Request{ctx: ctx, cache: newReqCache(fn)}
}

func newReqCache(load func() *shared.Request) *reqCache {
	return &reqCache{load: load}
}

type reqCache struct {
	loadOnce sync.Once
	load     func() *shared.Request
	req      *shared.Request
}

func (r *reqCache) Get() *shared.Request {
	r.loadOnce.Do(func() {
		if r.load == nil {
			r.req = &shared.Request{Type: shared.None}
			return
		}
		r.req = r.load()
		if r.req == nil {
			r.req = &shared.Request{Type: shared.None}
		}
	})
	return r.req
}
