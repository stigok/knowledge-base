package main

import (
	"fmt"
	"sort"
	"strings"
)

const TagPathSeparator = "/"

type Node struct {
	Label    string
	Value    any
	Parent   *Node `json:"-"`
	Children []*Node
}

// Returns the root node of the tree
func (tree *Node) Root() *Node {
	for tree.Parent != nil {
		tree = tree.Parent
	}
	return tree
}

// FullName joins all parent's labels with the current node's label, separated
// by `TagPathSeparator`.
func (node *Node) FullName() string {
	if node.Parent == nil {
		return node.Label
	}

	names := []string{}
	child := node
	for child.Parent != nil {
		names = append(names, child.Label)
		child = child.Parent
	}

	sort.Sort(sort.Reverse(sort.StringSlice(names)))
	return strings.Join(names, TagPathSeparator)
}

// Search returns a child node by its `FullName`. The node labels must be
// separated by `TagPathSeparator`.
func (tree *Node) Search(k string) *Node {
	if tree.Children == nil {
		return nil
	}

	for _, node := range tree.Children {
		if node.FullName() == k {
			return node
		}
		if m := node.Search(k); m != nil {
			return m
		}
	}

	return nil
}

// NewOrExisting returns an existing node with the specified `FullName` under
// the current node, or creates it if it does not exist.
func (tree *Node) NewOrExisting(k string) *Node {
	parts := strings.Split(k, TagPathSeparator)
	parent := tree

	for i := 0; i < len(parts); i++ {
		nk := strings.Join(parts[:i+1], TagPathSeparator)
		if node := parent.Search(nk); node != nil {
			parent = node
		} else {
			node = new(Node)
			node.Label = parts[i]
			node.Parent = parent
			parent.Children = append(parent.Children, node)
			parent = node
		}
	}

	return parent
}

// BuildTree builds a node tree with values from a map of `FullName`s.
func BuildTree[V any](m map[string]V) *Node {
	// Keep dir sorted. Important for the directory structure.
	var keys []string
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Create a new root node
	tree := new(Node)
	for _, k := range keys {
		node := tree.NewOrExisting(k)
		node.Value = m[k]
	}

	return tree
}

// UnpackHTMLOptions options for `UnpackHTML`.
type UnpackHTMLOptions struct {
	DirClass  string
	PostClass string
}

// UnpackHTML recursively unpacks the tree into a list of lists. Call this
// function with `indentLevel` set to `0`.
func (tree *Node) UnpackHTML(indentLevel int, opts *UnpackHTMLOptions) {
	if opts == nil {
		opts = new(UnpackHTMLOptions)
	}

	// Prettify HTML by indenting lines
	// TODO: Something wrong with the indenting, but can't be bothered.
	prefix := ""
	for i := 0; i < indentLevel; i++ {
		prefix += "  "
	}

	// If the indent level is 0, it's the root node. I don't want to print it.
	if indentLevel > 0 {
		// Name of the current directory
		fmt.Printf("%s<ul>\n", prefix)
		fmt.Printf("%s  <li class=\"%s\">%s\n", prefix, opts.DirClass, tree.Label)
	}

	// List directories first
	for _, child := range tree.Children {
		child.UnpackHTML(indentLevel+1, opts)
	}

	// Then print files within the directory
	if tree.Value != nil {
		fmt.Printf("%s    <ul>\n", prefix)
		for _, v := range tree.Value.([]*Post) {
			fmt.Printf("%s      <li class=\"%s\">%s</li>\n", prefix, opts.PostClass, v)
		}
		fmt.Printf("%s    </ul>\n", prefix)
	}

	if indentLevel > 0 {
		fmt.Printf("%s  </li>\n", prefix)
		fmt.Printf("%s</ul>\n", prefix)
	}
}

type Post struct {
	Name string
}

func (p *Post) String() string {
	return p.Name
}

func main() {
	values := map[string][]*Post{
		"foo": []*Post{
			&Post{"foo1"}, &Post{"foo2"},
		},
		"fight": []*Post{
			&Post{"fight1"}, &Post{"fight2"},
		},
		"foo/bar": []*Post{
			&Post{"nestedfoobar1"}, &Post{"nestedfoobar2"},
		},
		"foo/bar/baz": []*Post{
			&Post{"nested-foo-bar-baz-1"}, &Post{"nested-foo-bar-baz-2"},
		},
	}

	tree := BuildTree(values)
	tree.Label = ""
	tree.UnpackHTML(0, nil)
}
