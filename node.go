package main

import (
	"sort"
	"strings"
)

const TagPathSeparator = "/"

type Node struct {
	Label    string
	Value    []*Post
	Children []*Node
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

		// TODO: this can probably be (re)moved
		if match != nil {
			parent = match
			continue
		}

		child := new(Node)
		parent.Children = append(parent.Children, child)
		child.Label = parts[i]
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
