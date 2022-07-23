package main

import (
	"fmt"
	"sort"
	"strings"
)

const TagPathSeparator = "/"

type Node struct {
	Label    string
	Value    []*Post
	Parent   *Node `json:"-"`
	Children []*Node
}

// Root returns the root node of the tree.
func (tree *Node) Root() *Node {
	for tree.Parent != nil {
		tree = tree.Parent
	}
	return tree
}

// FullName returns the full path of the node, starting from the root.
func (node *Node) FullName() string {
	names := []string{}
	cur := node
	for {
		names = append([]string{cur.Label}, names...)
		if cur.Parent == nil {
			break
		}
		cur = cur.Parent
	}

	return strings.Join(names, TagPathSeparator)
}

// Search finds a node in the tree at path `k` under the current node.
func (node *Node) Search(k string) *Node {
	var match *Node
	keys := strings.Split(k, TagPathSeparator)

outer:
	for _, k := range keys {
		for _, child := range node.Children {
			if child.Label == k {
				match = child
			} else {
				match = nil
				break outer
			}
		}

		if match != nil {
			node = match
		}
	}

	return match
}

// NewOrExisting returns an existing node at the specified path under
// the current node, or creates it if it does not exist.
// TODO: don't require path segment for the source node
func (tree *Node) NewOrExisting(k string) *Node {
	parent := tree
	parts := strings.Split(k, TagPathSeparator)
	for i := 0; i < len(parts); i++ {
		if i == 0 && parent.Label == parts[0] {
			continue
		}

		var match *Node
		for _, child := range parent.Children {
			if child.Label == parts[i] {
				match = child
				break
			}
		}

		if match != nil {
			parent = match
			continue
		}

		child := new(Node)
		parent.Children = append(parent.Children, child)
		child.Label = parts[i]
		child.Parent = parent
		parent = child
	}

	return parent
}

// BuildTree builds a node tree with values from a map of `FullName`s.
// The root of the tree has no label by default.
func BuildTree(m map[string][]*Post) *Node {
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

func (tree *Node) UnpackHTML(opts *UnpackHTMLOptions) string {
	if opts == nil {
		opts = new(UnpackHTMLOptions)
	}

	var html string
	html += fmt.Sprintf("<ul>\n")
	html += fmt.Sprintf("  <li class=\"%s\">%s\n", opts.DirClass, tree.Label)
	html += tree.unpackHTML(2, *opts)
	html += fmt.Sprintf("  </li>\n")
	html += fmt.Sprintf("</ul>\n")
	return html
}

// UnpackHTML recursively unpacks the tree into a list of lists. Call this
// function with `indentLevel` set to `0`.
func (tree *Node) unpackHTML(indentLevel int, opts UnpackHTMLOptions) string {
	var (
		html   string
		prefix string = strings.Repeat("  ", indentLevel)
	)

	// Name of the current directory
	html += fmt.Sprintf("%s<ul>\n", prefix)
	html += fmt.Sprintf("%s  <li class=\"%s\">%s\n", prefix, opts.DirClass, tree.Label)

	// List directories first
	for _, child := range tree.Children {
		html += child.unpackHTML(indentLevel+2, opts)
	}

	// Then print files within the directory
	if tree.Value != nil {
		html += fmt.Sprintf("%s    <ul>\n", prefix)
		for _, v := range tree.Value {
			html += fmt.Sprintf("%s      <li class=\"%s\">%s</li>\n", prefix, opts.PostClass, v.Title)
		}
		html += fmt.Sprintf("%s    </ul>\n", prefix)
	}

	if indentLevel > 0 {
		html += fmt.Sprintf("%s  </li>\n", prefix)
		html += fmt.Sprintf("%s</ul>\n", prefix)
	}

	return html
}
