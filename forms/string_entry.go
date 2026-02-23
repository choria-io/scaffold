// Copyright (c) 2023-2024, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package forms

import (
	"fmt"
)

// newStringEntry creates a stringEntry initialized with the given string value.
func newStringEntry(v string) entry {
	e := &stringEntry{}
	e.set(v)

	return e
}

// stringEntry is an entry node that holds a string value used as a map key.
// Its combinedValue wraps child values under that key: with one child it produces
// {val: childValue}, with multiple children it merges them into {val: {merged children}}.
type stringEntry struct {
	graph
	isSet bool
	val   string
}

func (s *stringEntry) addChild(e entry) (entry, error) {
	if _, ok := e.(*objEntry); !ok {
		return nil, fmt.Errorf("incompatible type, only object child values are supported")
	}

	return s.graph.addChild(e)
}

func (s *stringEntry) set(v any) error {
	strVal, ok := v.(string)
	if !ok {
		return fmt.Errorf("incompatible value")
	}

	s.val = strVal
	s.isSet = true

	return nil
}

func (s *stringEntry) value() (isSet bool, value any) {
	return s.isSet, s.val
}

func (s *stringEntry) combinedValue() (isSet bool, value any) {
	if !s.isSet {
		return false, nil
	}

	if len(s.children) == 1 {
		_, childVal := s.children[0].combinedValue()
		return true, map[string]any{s.val: childVal}
	}

	resultMap := map[string]any{}
	result := map[string]any{
		s.val: resultMap,
	}

	s.eachChild(func(e entry) {
		if isSet, val := e.combinedValue(); isSet {
			if mapVal, ok := val.(map[string]any); ok {
				for k := range mapVal {
					resultMap[k] = mapVal[k]
				}
			}
		}
	})

	return true, result
}

func (s *stringEntry) isEmptyValue() bool {
	return s.val == ""
}
