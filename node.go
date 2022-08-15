package main

import (
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
