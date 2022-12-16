package main

import (
	"os"
	"testing"
	"time"

	"github.com/matryer/is"
)

func TestPosts(t *testing.T) {
	is := is.New(t)

	tmpdir, err := os.MkdirTemp("", "posts")
	is.NoErr(err)

	svc := NewPostsService(tmpdir)

	t.Run("list return zero posts", func(t *testing.T) {
		is := is.New(t)
		posts, err := svc.ListPosts(nil)
		is.NoErr(err)
		is.Equal(len(posts), 0)
	})

	t.Run("create a new post", func(t *testing.T) {
		is := is.New(t)
		err := svc.CreatePost(&Post{
			Title:   "foo",
			Content: "bar",
			Tags:    []Tag{"a", "b", "c", "_dir:/foo"},
		})
		is.NoErr(err)
	})

	var postId string

	t.Run("list returns one posts", func(t *testing.T) {
		is := is.New(t)
		posts, err := svc.ListPosts(nil)
		is.NoErr(err)
		is.Equal(len(posts), 1)

		postId = posts[0].ID
	})

	t.Run("get the created post", func(t *testing.T) {
		is := is.New(t)
		post, err := svc.GetPost(postId)
		is.NoErr(err)
		is.Equal(post.Title, "foo")
		is.Equal(post.Content, "bar")
	})

	t.Run("update the post", func(t *testing.T) {
		is := is.New(t)
		post, err := svc.GetPost(postId)
		is.NoErr(err)

		post.Title = "foo1"
		post.Content = "bar1"
		err = svc.UpdatePost(post)
		is.NoErr(err)
	})

	t.Run("an updated post has different create and update timestamps", func(t *testing.T) {
		is := is.New(t)
		post, err := svc.GetPost(postId)
		is.NoErr(err)
		is.True(post.CreatedTime != time.Time{})       // time is set
		is.True(post.ModifiedTime != time.Time{})      // time is set
		is.True(post.CreatedTime != post.ModifiedTime) // created and modified is not eq
	})

	t.Run("get the updated post", func(t *testing.T) {
		is := is.New(t)
		post, err := svc.GetPost(postId)
		is.NoErr(err)
		is.Equal(post.Title, "foo1")
		is.Equal(post.Content, "bar1")
	})

	t.Run("create another post", func(t *testing.T) {
		is := is.New(t)
		err := svc.CreatePost(&Post{
			Title:   "alice",
			Content: "bob",
			Tags:    []Tag{"d", "e", "_dir:/foo/bar"},
		})
		is.NoErr(err)
	})

	t.Run("list all tags", func(t *testing.T) {
		is := is.New(t)
		tags, err := svc.ListTags(&ListTagOptions{
			// Don't list the _dir:* tags
			IgnoreFunctional: true,
		})
		is.NoErr(err)
		is.Equal(len(tags), 5)
		is.Equal(tags[0], Tag("a"))
		is.Equal(tags[1], Tag("b"))
		is.Equal(tags[2], Tag("c"))
		is.Equal(tags[3], Tag("d"))
		is.Equal(tags[4], Tag("e"))
	})

	t.Run("list tree", func(t *testing.T) {
		is := is.New(t)
		tree, err := svc.GetPostsFolderTree()
		is.NoErr(err)

		var (
			post *Post
			node *Node
		)

		// Path: /
		is.Equal(len(tree.Children), 1)

		// Path: /foo
		node = tree.Children[0]
		is.Equal(node.Label, "foo")
		is.Equal(len(node.Value), 1)
		post = node.Value[0]
		is.Equal(post.Title, "foo1")

		// Path: /foo/bar
		node = node.Children[0]
		is.Equal(node.Label, "bar")
		is.Equal(len(node.Value), 1)
		post = node.Value[0]
		is.Equal(post.Title, "alice")
	})
}
