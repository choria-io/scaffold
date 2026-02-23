// Copyright (c) 2023-2024, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package forms

import (
	"fmt"
)

// newArrayEntry creates an arrayEntry initialized with the given slice.
func newArrayEntry(v []any) entry {
	e := &arrayEntry{}
	e.set(v)

	return e
}

// arrayEntry is an entry node that holds a []any value. Its combinedValue
// copies its own elements and appends any child combined values to the slice.
type arrayEntry struct {
	graph
	isSet bool
	val   []any
}

func (s *arrayEntry) isEmptyValue() bool {
	return len(s.val) == 0
}

func (a *arrayEntry) value() (isSet bool, value any) {
	return a.isSet, a.val
}

func (a *arrayEntry) combinedValue() (isSet bool, value any) {
	result := make([]any, len(a.val))
	copy(result, a.val)

	a.eachChild(func(e entry) {
		_, val := e.combinedValue()
		result = append(result, val)
	})

	return len(result) > 0, result
}

func (a *arrayEntry) set(v any) error {
	sliceVal, ok := v.([]any)
	if !ok {
		return fmt.Errorf("incompatible value")
	}

	a.val = sliceVal
	a.isSet = true

	return nil
}
