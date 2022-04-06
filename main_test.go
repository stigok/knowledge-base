package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"testing"
	"time"

	"github.com/matryer/is"
)

func TestGetPost(t *testing.T) {
	is := is.New(t)

	dir, err := os.MkdirTemp("", "")
	is.NoErr(err)
	defer os.RemoveAll(dir)

	app := App{
		Root: dir,
	}

	f, err := os.CreateTemp(dir, "")
	is.NoErr(err)

	id := path.Base(f.Name())
	b, err := json.Marshal(Post{
		ID:      id,
		Title:   "title",
		Content: "content",
	})
	is.NoErr(err)

	_, err = f.Write(b)
	is.NoErr(err)

	p, err := app.GetPost(id)
	is.NoErr(err)
	is.True(p != nil)
	is.Equal(p.ID, id)
	is.Equal(p.Title, "title")
	is.Equal(p.Content, "content")
}

func TestListPosts(t *testing.T) {
	is := is.New(t)

	dir, err := os.MkdirTemp("", "")
	is.NoErr(err)
	defer os.RemoveAll(dir)

	app := App{
		Root: dir,
	}

	for i := 0; i < 10; i++ {
		_, err = app.CreatePost(Post{
			Title:   fmt.Sprintf("title%d", i),
			Content: fmt.Sprintf("content%d", i),
		})
		is.NoErr(err)
	}

	posts, err := app.ListPosts()
	is.NoErr(err)
	is.True(posts != nil)
	is.Equal(len(posts), 10)
	// TODO: they are not sorted by time for some reason.
	// - ksuid is supposed to be naturally sortable by time
	// - os.WalkDir is supposed to walk directories lexicographically
	//is.Equal(posts[0].Title, "title0")
	//is.Equal(posts[0].Content, "content0")
	//is.Equal(posts[1].Title, "title1")
	//is.Equal(posts[1].Content, "content1")
}

func TestCreatePost(t *testing.T) {
	is := is.New(t)

	dir, err := os.MkdirTemp("", "")
	is.NoErr(err)
	defer os.RemoveAll(dir)

	app := App{
		Root: dir,
	}

	p, err := app.CreatePost(Post{
		Title:   "title",
		Content: "content",
		Tags: []string{
			"foo",
			"bar",
		},
	})
	is.NoErr(err)
	is.True(p.ID != "")
	is.Equal(p.Title, "title")
	is.Equal(p.Content, "content")
	is.True(p.CreatedTime != time.Time{})
	is.True(p.ModifiedTime != time.Time{})

	is.Equal(len(p.Tags), 2)
	is.Equal(p.Tags[0], "foo")
	is.Equal(p.Tags[1], "bar")
}

func TestUpdatePost(t *testing.T) {
	is := is.New(t)

	dir, err := os.MkdirTemp("", "")
	is.NoErr(err)
	defer os.RemoveAll(dir)

	app := App{
		Root: dir,
	}

	p, err := app.CreatePost(Post{
		Title:   "title",
		Content: "content",
	})

	p.Title = "foo"
	p.Content = "bar"
	p, err = app.UpdatePost(*p)
	is.NoErr(err)
	is.Equal(p.Title, "foo")
	is.Equal(p.Content, "bar")
}

func TestHTTP(t *testing.T) {
	is := is.New(t)

	// Setup test app
	dir, err := os.MkdirTemp("", "")
	is.NoErr(err)
	defer os.RemoveAll(dir)

	app := App{
		Root: dir,
	}

	// Seed app with posts
	var posts []*Post
	for i := 0; i < 10; i++ {
		p, err := app.CreatePost(Post{
			Title:   fmt.Sprintf("title%d", i),
			Content: fmt.Sprintf("content%d", i),
		})
		is.NoErr(err)
		posts = append(posts, p)
	}

	testCases := []struct {
		Method                string
		URL                   string
		RequestBody           []byte
		StatusCode            int
		ResponseBodyValidator func([]byte) bool
	}{
		{
			http.MethodGet, "/", nil,
			200, func(b []byte) bool {
				return bytes.Equal(b, []byte("Welcome!"))
			},
		},
		//
		// Posts
		//
		{
			http.MethodGet, "/posts/" + posts[0].ID, nil,
			200, func(b []byte) bool {
				var p Post
				json.Unmarshal(b, &p)
				return p.ID != "" &&
					p.Title == "title0" &&
					p.Content == "content0" &&
					p.CreatedTime != time.Time{} &&
					p.ModifiedTime != time.Time{}
			},
		},
		{
			http.MethodGet, "/posts/", nil,
			200, func(b []byte) bool {
				var posts []*Post
				json.Unmarshal(b, &posts)
				return len(posts) == 10
			},
		},
		{
			http.MethodPost, "/posts/", nil,
			400, func(b []byte) bool {
				return bytes.Equal(b, []byte("Bad Request"))
			},
		},
		{
			http.MethodPost, "/posts/", []byte(`{"title": "Foo", "content": "Bar"}`),
			201, func(b []byte) bool {
				var p Post
				json.Unmarshal(b, &p)
				return p.ID != "" &&
					p.Title == "Foo" &&
					p.Content == "Bar" &&
					p.CreatedTime != time.Time{} &&
					p.ModifiedTime != time.Time{}
			},
		},
		{
			http.MethodPost, "/posts/", []byte(`{"title": "Foo", "content": "Bar", "tags": ["foo", "bar"]}`),
			201, func(b []byte) bool {
				var p Post
				json.Unmarshal(b, &p)
				return p.ID != "" &&
					p.Title == "Foo" &&
					p.Content == "Bar" &&
					p.CreatedTime != time.Time{} &&
					p.ModifiedTime != time.Time{} &&
					len(p.Tags) == 2 &&
					p.Tags[0] == "foo" &&
					p.Tags[1] == "bar"
			},
		},
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
			is.True(tc.ResponseBodyValidator(body))
		})
	}
}
