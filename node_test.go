package main

import (
	"testing"

	"github.com/matryer/is"
)

func TestRoot(t *testing.T) {
	n1 := &Node{Label: "n1"}
	n2 := &Node{Label: "n2", Parent: n1}
	n3 := &Node{Label: "n3", Parent: n2}

	if n2.Root() != n1 {
		t.Errorf("n1 not root of n2 (%v != %v)", n1, n2)
	}

	if n3.Root() != n1 {
		t.Errorf("n1 not root of n3 (%v != %v)", n1, n3)
	}
}

func TestFullName(t *testing.T) {
	is := is.New(t)

	n1 := &Node{Label: "n1"}
	n2 := &Node{Label: "n2", Parent: n1}
	n3 := &Node{Label: "n3", Parent: n2}

	n1.Children = []*Node{n2}
	n2.Children = []*Node{n3}

	is.Equal(n1.FullName(), "n1")
	is.Equal(n2.FullName(), "n1/n2")
	is.Equal(n3.FullName(), "n1/n2/n3")
}

func TestSearch(t *testing.T) {
	n1 := &Node{Label: "n1"}
	n2 := &Node{Label: "n2", Parent: n1}
	n3 := &Node{Label: "n3", Parent: n2}
	n1.Children = []*Node{n2}
	n2.Children = []*Node{n3}

	t.Run("search from root", func(t *testing.T) {
		is := is.New(t)

		is.Equal(n1.Search("n1"), nil)
		is.Equal(n1.Search("n2"), n2)
		is.Equal(n1.Search("n2/n3"), n3)
	})

	t.Run("search from a child should use the calling child as root", func(t *testing.T) {
		is := is.New(t)

		is.Equal(n2.Search("n2"), nil)
		is.Equal(n2.Search("n3"), n3)
	})
}

func TestNewOrExisting(t *testing.T) {
	n1 := &Node{Label: "n1"}
	n2 := &Node{Label: "n2", Parent: n1}
	n3 := &Node{Label: "n3", Parent: n2}
	n1.Children = []*Node{n2}
	n2.Children = []*Node{n3}

	t.Run("gets an existing node", func(t *testing.T) {
		is := is.New(t)

		is.Equal(n1.NewOrExisting("n1/n2/n3"), n3)
	})

	t.Run("creates a new child node", func(t *testing.T) {
		is := is.New(t)

		n4 := n1.NewOrExisting("n1/n2/n4")
		is.Equal(n4.Parent, n2)
		is.Equal(n2.Children[1], n4)
		is.Equal(n4.FullName(), "n1/n2/n4")
	})

	t.Run("creates a new child node under a child", func(t *testing.T) {
		is := is.New(t)

		n5 := n3.NewOrExisting("n5")
		is.Equal(n5.Parent, n3)
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
	is.Equal(foo.FullName(), "/foo")

	is.Equal(len(foo.Children), 1)
	foobar := foo.Children[0]
	is.Equal(foobar.Label, "bar")
	is.Equal(foobar.FullName(), "/foo/bar")
}
