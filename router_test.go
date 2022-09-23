package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/matryer/is"
)

func TestRouter(t *testing.T) {
	is := is.New(t)

	t.Run("404 on no matching routes", func(t *testing.T) {
		is := is.New(t)

		router := Router{}
		router.Get("^/$", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("hello"))
		}))

		req := httptest.NewRequest(http.MethodGet, "/nothing", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)
		resp := w.Result()
		body, err := io.ReadAll(resp.Body)
		is.NoErr(err)

		is.Equal(resp.StatusCode, 404)
		is.Equal(string(body), "404 page not found\n")
	})

	t.Run("a simple route matches", func(t *testing.T) {
		is := is.New(t)

		router := Router{}
		router.Get("^/$", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("hello"))
		}))

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)
		resp := w.Result()
		body, err := io.ReadAll(resp.Body)
		is.NoErr(err)

		is.Equal(resp.StatusCode, 200)
		is.Equal(string(body), "hello")
	})

	t.Run("named parameters are added to the context", func(t *testing.T) {
		is := is.New(t)

		router := Router{}
		router.Get(`^/(?P<name>[\w_-]+)/(?P<number>\d+)$`, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			name := r.Context().Value("name").(string)
			number := r.Context().Value("number").(string)
			w.Write([]byte(name + " " + number))
		}))

		req := httptest.NewRequest(http.MethodGet, "/fourtytwo/42", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)
		resp := w.Result()
		body, err := io.ReadAll(resp.Body)
		is.NoErr(err)

		is.Equal(resp.StatusCode, 200)
		is.Equal(string(body), "fourtytwo 42")
	})
}

func TestRouterMethods(t *testing.T) {
	t.Run("Get adds a route", func(t *testing.T) {
		is := is.New(t)
		router := Router{}
		is.Equal(len(router.routes), 0)

		router.Get("^/foo$", http.NotFoundHandler())

		is.Equal(len(router.routes), 1)
		is.Equal(router.routes[0].method, http.MethodGet)
		is.Equal(router.routes[0].handler, http.NotFoundHandler())
		is.True(router.routes[0].pattern != nil)
	})

	t.Run("Use adds a middleware route", func(t *testing.T) {
		is := is.New(t)
		router := Router{}
		router.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				//lint:ignore SA1029 n/a
				ctx := context.WithValue(r.Context(), "foo", "bar")
				next.ServeHTTP(w, r.WithContext(ctx))
			})
		})
		router.Get("^/$", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprintf(w, "hello, %s", r.Context().Value("foo").(string))
		}))

		is.Equal(len(router.middlewares), 1)
		is.Equal(len(router.routes), 1)

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)
		resp := w.Result()
		body, err := io.ReadAll(resp.Body)
		is.NoErr(err)

		is.Equal(string(body), "hello, bar")
		is.Equal(resp.StatusCode, 200)
	})
}
