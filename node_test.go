package main

import (
	"testing"

	"github.com/matryer/is"
)

func TestNewOrExisting(t *testing.T) {
	t.Run("creates a new child node", func(t *testing.T) {
		is := is.New(t)

		a := &Node{Label: "a"}
		c := a.NewOrExisting("b/c")

		is.Equal(a.Children[0].Children[0], c)
	})

	t.Run("gets an existing node", func(t *testing.T) {
		is := is.New(t)

		a := &Node{Label: "a"}
		_ = a.NewOrExisting("b/c")
		b := a.NewOrExisting("b")

		is.Equal(a.Children[0], b)
	})

	t.Run("returns itself on empty key", func(t *testing.T) {
		is := is.New(t)
		a1 := &Node{Label: "a"}
		a2 := a1.NewOrExisting("")
		is.Equal(a1, a2)
	})

	t.Run("returns itself on a single TagPathSeparator", func(t *testing.T) {
		is := is.New(t)
		a1 := &Node{Label: "a"}
		a2 := a1.NewOrExisting(TagPathSeparator)
		is.Equal(a1, a2)
	})
}

func TestBuildTree(t *testing.T) {
	is := is.New(t)

	values := map[string][]*Post{
		"foo": []*Post{
			&Post{Title: "a"}, &Post{Title: "b"},
		},
		"foo/bar": []*Post{
			&Post{Title: "c"}, &Post{Title: "d"},
		},
	}

	tree := BuildTree(values)

	is.Equal(len(tree.Children), 1)
	foo := tree.Children[0]
	is.Equal(foo.Label, "foo")

	is.Equal(len(foo.Children), 1)
	foobar := foo.Children[0]
	is.Equal(foobar.Label, "bar")
}
