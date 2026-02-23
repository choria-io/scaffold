// Copyright (c) 2023-2024, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package forms

import (
	"fmt"
)

// newObjectEntry creates an objEntry initialized with the given map value.
func newObjectEntry(v map[string]any) entry {
	e := &objEntry{}
	e.set(v)

	return e
}

// objEntry is an entry node that holds a map[string]any value. When arrayMode is true
// (set by adding an arrayEntry child), combinedValue delegates to arrayModeCombined
// which wraps the child array under this object's key. Otherwise objModeCombined
// merges all child maps into a single result under this object's key.
type objEntry struct {
	graph
	isSet     bool
	val       map[string]any
	arrayMode bool
}

func (s *objEntry) isEmptyValue() bool {
	return len(s.val) == 0
}

func (o *objEntry) addChild(e entry) (entry, error) {
	switch e.(type) {
	case *objEntry, *stringEntry:
		return o.graph.addChild(e)

	case *arrayEntry:
		if o.hasChildren() {
			return nil, fmt.Errorf("only one array child is supported")
		}

		o.arrayMode = true

		return o.graph.addChild(e)

	default:
		return e, fmt.Errorf("incompatible child type")
	}
}

func (o *objEntry) value() (isSet bool, value any) {
	return o.isSet, o.val
}

func (o *objEntry) arrayModeCombined(key string) (isSet bool, value any) {
	res := map[string]any{}

	isSet, res[key] = o.children[0].combinedValue()

	return isSet, res
}

func (o *objEntry) objModeCombined(key string) (isSet bool, value any) {
	result := map[string]any{}
	resultMap := map[string]any{}
	if key == "" {
		result = resultMap
	} else {
		result[key] = resultMap
	}

	childValues := []map[string]any{}

	o.eachChild(func(e entry) {
		_, childVal := e.combinedValue()
		if m, ok := childVal.(map[string]any); ok {
			childValues = append(childValues, m)
		}
	})

	for _, e := range childValues {
		for k, v := range e {
			resultMap[k] = v
		}
	}

	return true, result
}

func (o *objEntry) combinedValue() (isSet bool, value any) {
	if !o.hasChildren() {
		return o.isSet, o.val
	}

	if !o.isSet {
		return false, o.val
	}

	key := ""
	for k := range o.val {
		key = k
		break
	}

	if o.arrayMode {
		return o.arrayModeCombined(key)
	} else {
		return o.objModeCombined(key)
	}
}

func (o *objEntry) set(v any) error {
	mapVal, ok := v.(map[string]any)
	if !ok {
		return fmt.Errorf("incompatible value")
	}

	o.val = mapVal
	o.isSet = true

	return nil
}
