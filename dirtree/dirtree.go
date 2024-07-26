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
