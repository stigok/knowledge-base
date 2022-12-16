package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/matryer/is"
)

func TestHTTP(t *testing.T) {
	is := is.New(t)

	// Setup test app
	dir, err := os.MkdirTemp("", "")
	is.NoErr(err)
	defer os.RemoveAll(dir)

	app := NewApp(dir, ":1337")

	// Seed app with posts
	var posts []*Post
	for i := 0; i < 10; i++ {
		p := &Post{
			Title:   fmt.Sprintf("title%d", i),
			Content: fmt.Sprintf("content%d", i),
			Tags:    []Tag{Tag(fmt.Sprintf("tag%d", i))},
		}
		err := app.posts.CreatePost(p)
		is.NoErr(err)
		posts = append(posts, p)
	}

	testCases := []struct {
		Method      string
		URL         string
		RequestBody []byte
		StatusCode  int
	}{
		{
			http.MethodGet, "/", nil,
			200,
		},
		//
		// Posts
		//
		{
			http.MethodGet, "/posts/" + posts[0].ID, nil,
			200,
		},
		{
			http.MethodGet, "/posts/", nil,
			200,
		},
		{
			http.MethodPost, "/posts/", nil,
			400,
		},
		{
			http.MethodPost, "/posts/", []byte(`{"title": "Foo", "content": "Bar"}`),
			201,
		},
		{
			http.MethodPost, "/posts/", []byte(`{"title": "Foo", "content": "Bar", "tags": ["foo", "bar"]}`),
			201,
		},
		//
		// Tags
		//
		//{
		//	http.MethodGet, "/tags/", nil,
		//	200,
		//},
	}

	for i, tc := range testCases {
		t.Run(fmt.Sprintf("%d %s %s", i, tc.Method, tc.URL), func(t *testing.T) {
			is := is.New(t)

			r := httptest.NewRequest(tc.Method, tc.URL, bytes.NewReader(tc.RequestBody))
			w := httptest.NewRecorder()
			app.ServeHTTP(w, r)

			resp := w.Result()
			body, err := io.ReadAll(resp.Body)
			is.NoErr(err)

			is.Equal(resp.StatusCode, tc.StatusCode)
			t.Log("Body:", string(body))
		})
	}
}
