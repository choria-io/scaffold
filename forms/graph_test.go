// Copyright (c) 2023-2024, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package forms

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestBuilder(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Forms")
}

var _ = Describe("Forms", func() {
	Describe("Graph", func() {
		It("Should generate correct values", func() {
			root := newObjectEntry(map[string]any{})
			root.addChild(newObjectEntry(map[string]any{"listen": "localhost:-1"}))

			ln, _ := root.addChild(newObjectEntry(map[string]any{"leafnode": nil}))
			ln.addChild(newObjectEntry(map[string]any{"credentials": "/x.cred"}))
			ln.addChild(newObjectEntry(map[string]any{"url": "connect.ngs.global:4222"}))
			urls, _ := ln.addChild(newObjectEntry(map[string]any{"urls": []any{}}))
			urls.addChild(newArrayEntry([]any{"x", "y"}))

			accounts, _ := root.addChild(newObjectEntry(map[string]any{"accounts": nil}))
			users, _ := accounts.addChild(newStringEntry("USERS"))
			uc, _ := users.addChild(newObjectEntry(map[string]any{"users": []any{}}))
			uc.addChild(newArrayEntry([]any{
				map[string]any{"username": "bob", "password": "b0b"},
				map[string]any{"username": "jill", "password": "j1ll"},
			}))

			expected := map[string]any{
				"accounts": map[string]any{
					"USERS": map[string]any{
						"users": []any{
							map[string]any{"password": "b0b", "username": "bob"},
							map[string]any{"password": "j1ll", "username": "jill"},
						},
					},
				},
				"leafnode": map[string]any{
					"credentials": "/x.cred",
					"url":         "connect.ngs.global:4222",
					"urls":        []any{"x", "y"},
				},
				"listen": "localhost:-1",
			}

			_, v := root.combinedValue()

			Expect(v).To(Equal(expected))
		})
	})

	Describe("graph base", func() {
		It("Should reject setting parent twice", func() {
			e := newObjectEntry(map[string]any{"a": 1})
			Expect(e.setParent(e)).To(Succeed())
			Expect(e.setParent(e)).To(MatchError("parent already set"))
		})
	})

	Describe("objEntry", func() {
		Describe("newObjectEntry", func() {
			It("Should create an entry with the given value", func() {
				e := newObjectEntry(map[string]any{"key": "val"})
				isSet, v := e.value()
				Expect(isSet).To(BeTrue())
				Expect(v).To(Equal(map[string]any{"key": "val"}))
			})
		})

		Describe("isEmptyValue", func() {
			It("Should be true for empty maps", func() {
				e := newObjectEntry(map[string]any{})
				Expect(e.isEmptyValue()).To(BeTrue())
			})

			It("Should be false for non-empty maps", func() {
				e := newObjectEntry(map[string]any{"k": "v"})
				Expect(e.isEmptyValue()).To(BeFalse())
			})
		})

		Describe("set", func() {
			It("Should accept map values", func() {
				e := newObjectEntry(map[string]any{})
				Expect(e.set(map[string]any{"new": "val"})).To(Succeed())
				isSet, v := e.value()
				Expect(isSet).To(BeTrue())
				Expect(v).To(Equal(map[string]any{"new": "val"}))
			})

			It("Should reject non-map values", func() {
				e := newObjectEntry(map[string]any{})
				Expect(e.set("string")).To(MatchError("incompatible value"))
				Expect(e.set(123)).To(MatchError("incompatible value"))
			})
		})

		Describe("addChild", func() {
			It("Should accept objEntry children", func() {
				parent := newObjectEntry(map[string]any{})
				child, err := parent.addChild(newObjectEntry(map[string]any{"a": 1}))
				Expect(err).ToNot(HaveOccurred())
				Expect(child).ToNot(BeNil())
			})

			It("Should accept stringEntry children", func() {
				parent := newObjectEntry(map[string]any{})
				child, err := parent.addChild(newStringEntry("key"))
				Expect(err).ToNot(HaveOccurred())
				Expect(child).ToNot(BeNil())
			})

			It("Should accept an arrayEntry child when no children exist", func() {
				parent := newObjectEntry(map[string]any{"items": []any{}})
				child, err := parent.addChild(newArrayEntry([]any{"a", "b"}))
				Expect(err).ToNot(HaveOccurred())
				Expect(child).ToNot(BeNil())
			})

			It("Should reject a second array child", func() {
				parent := newObjectEntry(map[string]any{"items": []any{}})
				_, err := parent.addChild(newArrayEntry([]any{"a"}))
				Expect(err).ToNot(HaveOccurred())

				_, err = parent.addChild(newArrayEntry([]any{"b"}))
				Expect(err).To(MatchError("only one array child is supported"))
			})

			It("Should reject array child when object children exist", func() {
				parent := newObjectEntry(map[string]any{})
				_, err := parent.addChild(newObjectEntry(map[string]any{"a": 1}))
				Expect(err).ToNot(HaveOccurred())

				_, err = parent.addChild(newArrayEntry([]any{"b"}))
				Expect(err).To(MatchError("only one array child is supported"))
			})
		})

		Describe("combinedValue", func() {
			It("Should return own value when there are no children", func() {
				e := newObjectEntry(map[string]any{"key": "val"})
				isSet, v := e.combinedValue()
				Expect(isSet).To(BeTrue())
				Expect(v).To(Equal(map[string]any{"key": "val"}))
			})

			It("Should return not-set when it has children but is not set", func() {
				e := &objEntry{}
				e.graph.addChild(newObjectEntry(map[string]any{"a": 1}))

				isSet, _ := e.combinedValue()
				Expect(isSet).To(BeFalse())
			})

			It("Should merge object children under the key", func() {
				parent := newObjectEntry(map[string]any{"server": nil})
				parent.addChild(newObjectEntry(map[string]any{"host": "localhost"}))
				parent.addChild(newObjectEntry(map[string]any{"port": 8080}))

				isSet, v := parent.combinedValue()
				Expect(isSet).To(BeTrue())
				Expect(v).To(Equal(map[string]any{
					"server": map[string]any{
						"host": "localhost",
						"port": 8080,
					},
				}))
			})

			It("Should merge children directly when key is empty", func() {
				root := newObjectEntry(map[string]any{})
				root.addChild(newObjectEntry(map[string]any{"a": 1}))
				root.addChild(newObjectEntry(map[string]any{"b": 2}))

				isSet, v := root.combinedValue()
				Expect(isSet).To(BeTrue())
				Expect(v).To(Equal(map[string]any{"a": 1, "b": 2}))
			})

			It("Should wrap array child under the key", func() {
				parent := newObjectEntry(map[string]any{"tags": []any{}})
				parent.addChild(newArrayEntry([]any{"x", "y", "z"}))

				isSet, v := parent.combinedValue()
				Expect(isSet).To(BeTrue())
				Expect(v).To(Equal(map[string]any{
					"tags": []any{"x", "y", "z"},
				}))
			})
		})
	})

	Describe("stringEntry", func() {
		Describe("newStringEntry", func() {
			It("Should create an entry with the given value", func() {
				e := newStringEntry("mykey")
				isSet, v := e.value()
				Expect(isSet).To(BeTrue())
				Expect(v).To(Equal("mykey"))
			})
		})

		Describe("isEmptyValue", func() {
			It("Should be true for empty strings", func() {
				e := newStringEntry("")
				Expect(e.isEmptyValue()).To(BeTrue())
			})

			It("Should be false for non-empty strings", func() {
				e := newStringEntry("x")
				Expect(e.isEmptyValue()).To(BeFalse())
			})
		})

		Describe("set", func() {
			It("Should accept string values", func() {
				e := newStringEntry("")
				Expect(e.set("updated")).To(Succeed())
				_, v := e.value()
				Expect(v).To(Equal("updated"))
			})

			It("Should reject non-string values", func() {
				e := newStringEntry("")
				Expect(e.set(123)).To(MatchError("incompatible value"))
				Expect(e.set(map[string]any{})).To(MatchError("incompatible value"))
			})
		})

		Describe("addChild", func() {
			It("Should accept objEntry children", func() {
				parent := newStringEntry("key")
				child, err := parent.addChild(newObjectEntry(map[string]any{"a": 1}))
				Expect(err).ToNot(HaveOccurred())
				Expect(child).ToNot(BeNil())
			})

			It("Should reject non-objEntry children", func() {
				parent := newStringEntry("key")
				_, err := parent.addChild(newStringEntry("other"))
				Expect(err).To(MatchError("incompatible type, only object child values are supported"))

				_, err = parent.addChild(newArrayEntry([]any{"a"}))
				Expect(err).To(MatchError("incompatible type, only object child values are supported"))
			})
		})

		Describe("combinedValue", func() {
			It("Should return not-set when the string is not set", func() {
				e := &stringEntry{}
				isSet, v := e.combinedValue()
				Expect(isSet).To(BeFalse())
				Expect(v).To(BeNil())
			})

			It("Should wrap a single child value under the key", func() {
				parent := newStringEntry("namespace")
				parent.addChild(newObjectEntry(map[string]any{"port": 8080}))

				isSet, v := parent.combinedValue()
				Expect(isSet).To(BeTrue())
				Expect(v).To(Equal(map[string]any{
					"namespace": map[string]any{"port": 8080},
				}))
			})

			It("Should merge multiple children under the key", func() {
				parent := newStringEntry("ns")
				parent.addChild(newObjectEntry(map[string]any{"a": 1}))
				parent.addChild(newObjectEntry(map[string]any{"b": 2}))

				isSet, v := parent.combinedValue()
				Expect(isSet).To(BeTrue())
				Expect(v).To(Equal(map[string]any{
					"ns": map[string]any{"a": 1, "b": 2},
				}))
			})

			It("Should skip unset children when merging", func() {
				parent := newStringEntry("ns")
				child := &objEntry{}
				parent.(*stringEntry).graph.addChild(child)
				parent.addChild(newObjectEntry(map[string]any{"a": 1}))

				isSet, v := parent.combinedValue()
				Expect(isSet).To(BeTrue())
				Expect(v).To(Equal(map[string]any{
					"ns": map[string]any{"a": 1},
				}))
			})
		})
	})

	Describe("arrayEntry", func() {
		Describe("newArrayEntry", func() {
			It("Should create an entry with the given value", func() {
				e := newArrayEntry([]any{"a", "b"})
				isSet, v := e.value()
				Expect(isSet).To(BeTrue())
				Expect(v).To(Equal([]any{"a", "b"}))
			})
		})

		Describe("isEmptyValue", func() {
			It("Should be true for empty slices", func() {
				e := newArrayEntry([]any{})
				Expect(e.isEmptyValue()).To(BeTrue())
			})

			It("Should be false for non-empty slices", func() {
				e := newArrayEntry([]any{"x"})
				Expect(e.isEmptyValue()).To(BeFalse())
			})
		})

		Describe("set", func() {
			It("Should accept slice values", func() {
				e := newArrayEntry([]any{})
				Expect(e.set([]any{"new"})).To(Succeed())
				_, v := e.value()
				Expect(v).To(Equal([]any{"new"}))
			})

			It("Should reject non-slice values", func() {
				e := newArrayEntry([]any{})
				Expect(e.set("string")).To(MatchError("incompatible value"))
				Expect(e.set(123)).To(MatchError("incompatible value"))
			})
		})

		Describe("combinedValue", func() {
			It("Should return not-set for empty arrays with no children", func() {
				e := newArrayEntry([]any{})
				isSet, v := e.combinedValue()
				Expect(isSet).To(BeFalse())
				Expect(v).To(BeEmpty())
			})

			It("Should return own values when there are no children", func() {
				e := newArrayEntry([]any{"a", "b"})
				isSet, v := e.combinedValue()
				Expect(isSet).To(BeTrue())
				Expect(v).To(Equal([]any{"a", "b"}))
			})

			It("Should not mutate the original slice", func() {
				e := newArrayEntry([]any{"a"})
				e.combinedValue()

				_, v := e.value()
				Expect(v).To(Equal([]any{"a"}))
			})
		})
	})

	Describe("partial result querying", func() {
		It("Should reflect accumulated state during tree construction", func() {
			root := newObjectEntry(map[string]any{})

			root.addChild(newObjectEntry(map[string]any{"first": "one"}))
			isSet, v := root.combinedValue()
			Expect(isSet).To(BeTrue())
			Expect(v).To(Equal(map[string]any{"first": "one"}))

			root.addChild(newObjectEntry(map[string]any{"second": "two"}))
			_, v = root.combinedValue()
			Expect(v).To(Equal(map[string]any{"first": "one", "second": "two"}))

			nested, _ := root.addChild(newObjectEntry(map[string]any{"nested": nil}))
			nested.addChild(newObjectEntry(map[string]any{"deep": "value"}))
			_, v = root.combinedValue()
			Expect(v).To(Equal(map[string]any{
				"first":  "one",
				"second": "two",
				"nested": map[string]any{"deep": "value"},
			}))
		})
	})
})
