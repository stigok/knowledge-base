package main

import (
	"testing"

	"github.com/matryer/is"
)

func TestNewOrExisting(t *testing.T) {
	n1 := &Node{Label: "n1"}
	n2 := &Node{Label: "n2"}
	n3 := &Node{Label: "n3"}
	n1.Children = []*Node{n2}
	n2.Children = []*Node{n3}

	t.Run("gets an existing node", func(t *testing.T) {
		is := is.New(t)

		is.Equal(n1.NewOrExisting("n1/n2/n3"), n3)
	})

	t.Run("creates a new child node", func(t *testing.T) {
		is := is.New(t)

		n4 := n1.NewOrExisting("n1/n2/n4")
		is.Equal(n2.Children[1], n4)
	})

	t.Run("creates a new child node under a child", func(t *testing.T) {
		is := is.New(t)

		n5 := n3.NewOrExisting("n5")
		is.Equal(len(n3.Children), 1)
		is.Equal(n3.Children[0], n5)
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
