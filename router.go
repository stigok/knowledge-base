package main

import (
	"context"
	"net/http"
	"regexp"
)

type Router struct {
	routes      []Route
	middlewares []Middleware
}

// Get adds a route that matches the HTTP GET method.
func (r *Router) Get(pat string, handler http.Handler) {
	route := Route{
		method:  http.MethodGet,
		pattern: regexp.MustCompile(pat),
		handler: handler,
	}
	r.routes = append(r.routes, route)
}

// Get adds a route that matches the HTTP GET method.
func (r *Router) Post(pat string, handler http.Handler) {
	route := Route{
		method:  http.MethodPost,
		pattern: regexp.MustCompile(pat),
		handler: handler,
	}
	r.routes = append(r.routes, route)
}

// Get adds a route that matches the HTTP GET method.
func (r *Router) Use(handler func(http.Handler) http.Handler) {
	r.middlewares = append(r.middlewares, handler)
}

type Route struct {
	// HTTP method to match. Will match all methods if it is an empty string.
	method string
	// The URL path pattern to match. Will match all paths if `nil`.
	pattern *regexp.Regexp
	// The handler function to process the request.
	handler http.Handler
}

type Middleware func(http.Handler) http.Handler

func (router *Router) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var handler http.Handler

	// Find a matching route wihin the stack
	for _, route := range router.routes {
		if route.method != "" && route.method != r.Method {
			continue
		}

		m := route.pattern.FindStringSubmatch(r.URL.Path)
		if m == nil || (len(m) == 1 && m[0] == "") {
			continue
		}

		// Add regexp named capture groups as values to the request context
		if paramNames := route.pattern.SubexpNames(); len(paramNames) > 1 {
			ctx := r.Context()
			for _, param := range route.pattern.SubexpNames()[1:] {
				//lint:ignore SA1029 don't see an alternative right now
				ctx = context.WithValue(ctx, param, m[route.pattern.SubexpIndex(param)])
			}
			r = r.WithContext(ctx)
		}

		handler = route.handler
		break
	}

	// If no routes matched, default to 404, but still let the middleware execute
	if handler == nil {
		handler = http.NotFoundHandler()
	}

	// If there's no middleware, call the handler immediately
	if len(router.middlewares) == 0 {
		handler.ServeHTTP(w, r)
		return
	}

	// Call the middleware functions in reverse order to create the correct stack
	mw := router.middlewares[len(router.middlewares)-1](handler)
	for i := len(router.middlewares) - 2; i >= 0; i-- {
		mw = router.middlewares[i](mw)
	}
	mw.ServeHTTP(w, r)
}
