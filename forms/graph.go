// Copyright (c) 2023-2024, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package forms

import (
	"fmt"
)

// entry defines the interface for nodes in the result tree.
//
// The tree is built incrementally as the user answers form questions. Conditional
// properties need to evaluate expressions against earlier answers while the tree is
// still being constructed, so combinedValue can be called at any point to walk the
// tree and produce the result accumulated so far.
//
// Three concrete types implement entry:
//   - objEntry: holds a map[string]any, merges children under its key
//   - stringEntry: holds a string used as a map key, wraps children under it
//   - arrayEntry: holds a []any, appends child values to its slice
//
// The first return value of value and combinedValue indicates whether the node
// has been set (true = value is present and valid).
type entry interface {
	addChild(entry) (entry, error)
	setParent(entry) error
	value() (isSet bool, value any)
	combinedValue() (isSet bool, value any)
	set(any) error
	isEmptyValue() bool
}

// graph provides the shared tree structure embedded by all entry types.
// It maintains an ordered list of children and a parent guard that prevents
// a node from being added to more than one parent.
type graph struct {
	children []entry
	parent   entry
}

// addChild appends e as a child of this node. It calls setParent on the child
// to enforce the single-parent invariant. The concrete entry types (objEntry,
// stringEntry) override addChild to enforce type constraints before delegating here.
func (g *graph) addChild(e entry) (entry, error) {
	err := e.setParent(e)
	if err != nil {
		return nil, err
	}

	g.children = append(g.children, e)

	return e, nil
}

// setParent marks this node as having been added to a parent. It returns an error
// if the node has already been adopted, preventing it from appearing in multiple
// places in the tree. The stored value is not used for traversal.
func (g *graph) setParent(e entry) error {
	if g.parent != nil {
		return fmt.Errorf("parent already set")
	}

	g.parent = e

	return nil
}

// hasChildren reports whether this node has any children.
func (g *graph) hasChildren() bool {
	return len(g.children) > 0
}

// eachChild calls cb for each child in insertion order.
func (g *graph) eachChild(cb func(entry)) {
	for i := 0; i < len(g.children); i++ {
		cb(g.children[i])
	}
}
