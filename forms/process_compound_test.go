// Copyright (c) 2023-2024, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package forms

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
)

var _ = Describe("ProcessForm compound types", func() {
	var (
		ctrl *gomock.Controller
		mock *Mocksurveyor
		opts []processOption
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		mock = NewMocksurveyor(ctrl)
		opts = testOpts(mock)
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("Object with sub-properties (type empty)", func() {
		It("Should gather sub-properties under the object name", func() {
			f := Form{
				Description: "test",
				Properties: []Property{
					{
						Name:        "server",
						Description: "server config",
						Type:        "",
						Properties: []Property{
							{Name: "host", Description: "host", Type: StringType},
							{Name: "port", Description: "port", Type: IntType},
						},
					},
				},
			}

			// press enter to start
			mock.EXPECT().AskOne(gomock.Any(), gomock.Any()).Return(nil)
			// host string (not required, no validator -> 2 args)
			mockStringResponse(mock, "localhost")
			// port int (always has validator -> 3 args)
			mockStringResponseV(mock, "8080")

			res, err := ProcessForm(f, nil, opts...)
			Expect(err).ToNot(HaveOccurred())
			Expect(res).To(Equal(map[string]any{
				"server": map[string]any{
					"host": "localhost",
					"port": 8080,
				},
			}))
		})
	})

	Describe("Object type entries", func() {
		It("Should ask for unique name and sub-properties then decline", func() {
			f := Form{
				Description: "test",
				Properties: []Property{
					{
						Name:        "accounts",
						Description: "accounts",
						Type:        ObjectType,
						Required:    false,
						IfEmpty:     ObjectIfEmpty,
						Properties: []Property{
							{Name: "email", Description: "email", Type: StringType},
						},
					},
				},
			}

			gomock.InOrder(
				// press enter
				mock.EXPECT().AskOne(gomock.Any(), gomock.Any()).Return(nil),
				// "Add accounts entry" confirmation -> yes
				mockBoolResponse(mock, true),
				// unique name for entry (has Required validator -> 3 args)
				mockStringResponseV(mock, "admin"),
				// email for admin (not required, no validator -> 2 args)
				mockStringResponse(mock, "admin@test.com"),
				// "Add accounts entry" confirmation -> no
				mockBoolResponse(mock, false),
			)

			res, err := ProcessForm(f, nil, opts...)
			Expect(err).ToNot(HaveOccurred())
			// ObjectType entries are added to parent directly by unique name;
			// declining adds the empty val with the property name
			Expect(res).To(Equal(map[string]any{
				"admin":    map[string]any{"email": "admin@test.com"},
				"accounts": map[string]any{},
			}))
		})
	})

	Describe("Required object type entries", func() {
		It("Should require first entry without confirmation then ask for more", func() {
			f := Form{
				Description: "test",
				Properties: []Property{
					{
						Name:        "servers",
						Description: "servers",
						Type:        ObjectType,
						Required:    true,
						IfEmpty:     ObjectIfEmpty,
						Properties: []Property{
							{Name: "host", Description: "host", Type: StringType},
						},
					},
				},
			}

			gomock.InOrder(
				// press enter to start
				mock.EXPECT().AskOne(gomock.Any(), gomock.Any()).Return(nil),
				// NO confirmation for first entry (required)
				// unique name (has Required validator -> 3 args)
				mockStringResponseV(mock, "web1"),
				// host (not required -> 2 args)
				mockStringResponse(mock, "10.0.0.1"),
				// "Add servers entry" confirmation -> no
				mockBoolResponse(mock, false),
			)

			res, err := ProcessForm(f, nil, opts...)
			Expect(err).ToNot(HaveOccurred())
			Expect(res).To(Equal(map[string]any{
				"web1":    map[string]any{"host": "10.0.0.1"},
				"servers": map[string]any{},
			}))
		})

		It("Should allow multiple required entries then decline", func() {
			f := Form{
				Description: "test",
				Properties: []Property{
					{
						Name:        "servers",
						Description: "servers",
						Type:        ObjectType,
						Required:    true,
						IfEmpty:     ObjectIfEmpty,
						Properties: []Property{
							{Name: "host", Description: "host", Type: StringType},
						},
					},
				},
			}

			gomock.InOrder(
				// press enter to start
				mock.EXPECT().AskOne(gomock.Any(), gomock.Any()).Return(nil),
				// first entry: no confirmation (required)
				mockStringResponseV(mock, "web1"),
				mockStringResponse(mock, "10.0.0.1"),
				// "Add servers entry" confirmation -> yes
				mockBoolResponse(mock, true),
				// second entry
				mockStringResponseV(mock, "web2"),
				mockStringResponse(mock, "10.0.0.2"),
				// "Add servers entry" confirmation -> no
				mockBoolResponse(mock, false),
			)

			res, err := ProcessForm(f, nil, opts...)
			Expect(err).ToNot(HaveOccurred())
			Expect(res).To(Equal(map[string]any{
				"web1":    map[string]any{"host": "10.0.0.1"},
				"web2":    map[string]any{"host": "10.0.0.2"},
				"servers": map[string]any{},
			}))
		})
	})

	Describe("Optional object type", func() {
		It("Should return empty when declined", func() {
			f := Form{
				Description: "test",
				Properties: []Property{
					{
						Name:        "extras",
						Description: "extras",
						Type:        ObjectType,
						Required:    false,
						IfEmpty:     ObjectIfEmpty,
						Properties: []Property{
							{Name: "key", Description: "key", Type: StringType},
						},
					},
				},
			}

			gomock.InOrder(
				// press enter
				mock.EXPECT().AskOne(gomock.Any(), gomock.Any()).Return(nil),
				// "Add extras entry" confirmation -> no
				mockBoolResponse(mock, false),
			)

			res, err := ProcessForm(f, nil, opts...)
			Expect(err).ToNot(HaveOccurred())
			Expect(res).To(Equal(map[string]any{
				"extras": map[string]any{},
			}))
		})
	})

	Describe("String array", func() {
		It("Should collect required first entry then optional additional", func() {
			f := Form{
				Description: "test",
				Properties: []Property{
					{Name: "tags", Description: "tags", Type: ArrayType, Required: true},
				},
			}

			gomock.InOrder(
				// press enter
				mock.EXPECT().AskOne(gomock.Any(), gomock.Any()).Return(nil),
				// required first entry (Required=true -> has validator -> 3 args)
				mockStringResponseV(mock, "alpha"),
				// "Add additional" -> yes
				mockBoolResponse(mock, true),
				// second entry (still Required=true -> 3 args)
				mockStringResponseV(mock, "beta"),
				// "Add additional" -> no (empty string breaks)
				mockBoolResponse(mock, false),
			)

			res, err := ProcessForm(f, nil, opts...)
			Expect(err).ToNot(HaveOccurred())
			Expect(res).To(Equal(map[string]any{
				"tags": []any{"alpha", "beta"},
			}))
		})

		It("Should handle optional array declining first entry", func() {
			f := Form{
				Description: "test",
				Properties: []Property{
					{Name: "tags", Description: "tags", Type: ArrayType, Required: false},
				},
			}

			gomock.InOrder(
				mock.EXPECT().AskOne(gomock.Any(), gomock.Any()).Return(nil),
				// "Add first" -> no
				mockBoolResponse(mock, false),
			)

			res, err := ProcessForm(f, nil, opts...)
			Expect(err).ToNot(HaveOccurred())
			Expect(res).To(Equal(map[string]any{
				"tags": []any{},
			}))
		})
	})

	Describe("Object array", func() {
		It("Should collect required first entry", func() {
			f := Form{
				Description: "test",
				Properties: []Property{
					{
						Name:        "users",
						Description: "users",
						Type:        ArrayType,
						Required:    true,
						Properties: []Property{
							{Name: "name", Description: "name", Type: StringType},
						},
					},
				},
			}

			gomock.InOrder(
				mock.EXPECT().AskOne(gomock.Any(), gomock.Any()).Return(nil),
				// first entry required, no confirmation
				mockStringResponse(mock, "bob"),
				// "Add additional" -> no
				mockBoolResponse(mock, false),
			)

			res, err := ProcessForm(f, nil, opts...)
			Expect(err).ToNot(HaveOccurred())
			Expect(res).To(Equal(map[string]any{
				"users": []any{
					map[string]any{"name": "bob"},
				},
			}))
		})

		It("Should handle optional object array with absent", func() {
			f := Form{
				Description: "test",
				Properties: []Property{
					{
						Name:        "items",
						Description: "items",
						Type:        ArrayType,
						Required:    false,
						IfEmpty:     AbsentIfEmpty,
						Properties: []Property{
							{Name: "v", Description: "v", Type: StringType},
						},
					},
				},
			}

			gomock.InOrder(
				mock.EXPECT().AskOne(gomock.Any(), gomock.Any()).Return(nil),
				// "Add first" -> no
				mockBoolResponse(mock, false),
			)

			res, err := ProcessForm(f, nil, opts...)
			Expect(err).ToNot(HaveOccurred())
			// absent means key should not appear, array is empty
			Expect(res).To(Equal(map[string]any{
				"items": []any{},
			}))
		})
	})

	Describe("Multiple properties", func() {
		It("Should handle mixed types in one form", func() {
			f := Form{
				Description: "test",
				Properties: []Property{
					{Name: "name", Description: "name", Type: StringType, Required: true},
					{Name: "age", Description: "age", Type: IntType},
					{Name: "active", Description: "active", Type: BoolType},
				},
			}

			gomock.InOrder(
				mock.EXPECT().AskOne(gomock.Any(), gomock.Any()).Return(nil),
				// name (Required=true -> validator -> 3 args)
				mockStringResponseV(mock, "alice"),
				// age (IntType -> always has validator -> 3 args)
				mockStringResponseV(mock, "30"),
				// active (BoolType -> no validator -> 2 args)
				mockBoolResponse(mock, true),
			)

			res, err := ProcessForm(f, nil, opts...)
			Expect(err).ToNot(HaveOccurred())
			Expect(res).To(Equal(map[string]any{
				"name":   "alice",
				"age":    30,
				"active": true,
			}))
		})
	})
})
