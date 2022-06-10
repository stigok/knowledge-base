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

	app := NewPostsService(dir)

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

	app := NewPostsService(dir)

	for i := 0; i < 10; i++ {
		err = app.CreatePost(&Post{
			Title:   fmt.Sprintf("title%d", i),
			Content: fmt.Sprintf("content%d", i),
		})
		is.NoErr(err)
	}

	posts, err := app.ListPosts(nil)
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

	app := NewPostsService(dir)

	p := &Post{
		Title:   "title",
		Content: "content",
		Tags: []string{
			"foo",
			"bar",
		},
	}
	err = app.CreatePost(p)
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

	app := NewPostsService(dir)

	p := &Post{
		Title:   "title",
		Content: "content",
	}
	err = app.CreatePost(p)
	is.NoErr(err)

	p.Title = "foo"
	p.Content = "bar"
	err = app.UpdatePost(p)
	is.NoErr(err)
	is.Equal(p.Title, "foo")
	is.Equal(p.Content, "bar")
}

//func TestListTags(t *testing.T) {
//	is := is.New(t)
//
//	dir, err := os.MkdirTemp("", "")
//	is.NoErr(err)
//	defer os.RemoveAll(dir)
//
//	app := NewPostsService(dir)
//
//	for i := 0; i < 10; i++ {
//		err = app.CreatePost(&Post{
//			Title:   fmt.Sprintf("title%d", i),
//			Content: fmt.Sprintf("content%d", i),
//			// Add one new tag per post, and one shared
//			Tags: []string{fmt.Sprintf("tag%d", i), "shared"},
//		})
//		is.NoErr(err)
//	}
//
//	tags, err := app.ListTags()
//	is.NoErr(err)
//	is.True(tags != nil)
//	is.Equal(len(tags), 11)
//	is.Equal(tags[0], "shared")
//	is.Equal(tags[1], "tag0")
//	is.Equal(tags[2], "tag1")
//}

func TestHTTP(t *testing.T) {
	is := is.New(t)

	// Setup test app
	dir, err := os.MkdirTemp("", "")
	is.NoErr(err)
	defer os.RemoveAll(dir)

	app := NewApp(dir, "/tmp/staticfiles", ":1337")

	// Seed app with posts
	var posts []*Post
	for i := 0; i < 10; i++ {
		p := &Post{
			Title:   fmt.Sprintf("title%d", i),
			Content: fmt.Sprintf("content%d", i),
			Tags:    []string{fmt.Sprintf("tag%d", i)},
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

func TestSearchAPI(t *testing.T) {
	is := is.New(t)

	// Setup test app
	dir, err := os.MkdirTemp("", "")
	is.NoErr(err)
	defer os.RemoveAll(dir)

	app := NewApp(dir, "/tmp/staticfiles", ":1337")

	// Seed app with posts
	var posts []*Post
	for i := 0; i < 10; i++ {
		p := &Post{
			Title:   fmt.Sprintf("title%d", i),
			Content: fmt.Sprintf("content%d", i),
			Tags:    []string{fmt.Sprintf("tag%d", i)},
		}
		err := app.posts.CreatePost(p)
		is.NoErr(err)
		posts = append(posts, p)
	}

	testCases := []struct {
		Method        string
		URL           string
		RequestBody   []byte
		StatusCode    int
		BodyValidator func(*testing.T, []byte)
	}{
		{
			http.MethodGet, "/api/search?q=content7", nil,
			200, func(t *testing.T, body []byte) {
				t.Helper()
				var resp []*Post
				err := json.NewDecoder(bytes.NewReader(body)).Decode(&resp)
				is.NoErr(err)
				is.Equal(len(resp), 1)
				is.Equal(resp[0].Title, "title7")
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
			tc.BodyValidator(t, body)
			t.Log("Body:", string(body))
		})
	}
}
