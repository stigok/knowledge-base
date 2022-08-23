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

// NewOrExisting returns an existing node, or creates a new, at the specified
// path. An empty key will return the current node. Path segments must be
// separated by `TagPathSeparator`. Leading and trailing separators are
// ignored.
func (node *Node) NewOrExisting(k string) *Node {
	if k = strings.Trim(k, TagPathSeparator); k == "" {
		return node
	}

	segs := strings.Split(k, TagPathSeparator)

	current, segs := segs[0], segs[1:]
	for _, child := range node.Children {
		if child.Label == current {
			return child.NewOrExisting(strings.Join(segs, TagPathSeparator))
		}
	}

	child := &Node{
		Label: current,
	}
	node.Children = append(node.Children, child)
	return child.NewOrExisting(strings.Join(segs, TagPathSeparator))
}

// BuildTree builds a node tree with values from a map.
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
