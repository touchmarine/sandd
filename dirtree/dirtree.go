package dirtree

import (
	"fmt"
	"io"
	"path/filepath"
	"slices"
	"strings"
)

// Node represents a dir or file in the directory tree.
type Node struct {
	Value    string // blank in root node
	Count    int    // number of times is part of a path
	Children []*Node
}

// Add creates a directory tree from the given path.
// Each node represents a path segment (separated by OS separator).
func (n *Node) Add(path string) {
	if path == "" {
		return
	}
	s := filepath.ToSlash(path) // i acknowledge this is done repeatedly per path
	before, after, found := strings.Cut(s, "/")
	segment := before
	if before == "" && found {
		// has leading slash; preserve it
		segment = "/"
	}

	var c *Node
	i := slices.IndexFunc(n.Children, func(x *Node) bool {
		return x.Value == segment
	})
	if i > -1 {
		// already exists
		c = n.Children[i]
	} else {
		c = &Node{
			Value: segment,
		}
		n.Children = append(n.Children, c)
	}
	c.Count++
	c.Add(after)
}

// Compressed returns a new radix/compressed trie.
// (Any node with only one child gets merged with its parent).
func (n *Node) Compressed() *Node {
	nn := &Node{
		Value: n.Value,
		Count: n.Count,
	}
	if len(n.Children) == 1 {
		c := n.Children[0]
		cc := c.Compressed()
		nn.Value = filepath.Join(nn.Value, cc.Value)
		nn.Count = cc.Count
		nn.Children = append(nn.Children, cc.Children...)
		return nn
	}
	nn.Children = make([]*Node, len(n.Children))
	for i, c := range n.Children {
		cc := c.Compressed()
		nn.Children[i] = cc
	}
	return nn
}

// WalkChildren traverses breadth-first and callbacks with the current node and
// children. It doesn't callback from the starting node.
func (n *Node) WalkChildren(fn func(cur *Node, children []*Node) bool) {
	if !fn(n, n.Children) {
		return
	}
	for _, c := range n.Children {
		c.WalkChildren(fn)
	}
}

// Print prints one node per line. Hierarchy is expressed with depth.
//
// line format: depth value count
func (n *Node) Print(w io.Writer, depth int) {
	fmt.Fprintln(w, depth, n.Value, n.Count)
	for _, c := range n.Children {
		c.Print(w, depth+1)
	}
}
