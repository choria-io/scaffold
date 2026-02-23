// Copyright (c) 2023-2024, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package forms

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
)

var _ = Describe("ProcessForm numeric types", func() {
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

	Describe("Integer properties", func() {
		It("Should process a valid integer", func() {
			f := Form{
				Description: "test",
				Properties: []Property{
					{Name: "count", Description: "how many", Type: IntType},
				},
			}

			mock.EXPECT().AskOne(gomock.Any(), gomock.Any()).Return(nil)
			mockStringResponseV(mock, "42")

			res, err := ProcessForm(f, nil, opts...)
			Expect(err).ToNot(HaveOccurred())
			Expect(res).To(Equal(map[string]any{"count": 42}))
		})
	})

	Describe("Float properties", func() {
		It("Should process a valid float", func() {
			f := Form{
				Description: "test",
				Properties: []Property{
					{Name: "rate", Description: "rate", Type: FloatType},
				},
			}

			mock.EXPECT().AskOne(gomock.Any(), gomock.Any()).Return(nil)
			mockStringResponseV(mock, "3.14")

			res, err := ProcessForm(f, nil, opts...)
			Expect(err).ToNot(HaveOccurred())
			Expect(res).To(Equal(map[string]any{"rate": 3.14}))
		})
	})

	Describe("Bool properties", func() {
		It("Should process true", func() {
			f := Form{
				Description: "test",
				Properties: []Property{
					{Name: "enabled", Description: "enable?", Type: BoolType},
				},
			}

			mock.EXPECT().AskOne(gomock.Any(), gomock.Any()).Return(nil)
			mockBoolResponse(mock, true)

			res, err := ProcessForm(f, nil, opts...)
			Expect(err).ToNot(HaveOccurred())
			Expect(res).To(Equal(map[string]any{"enabled": true}))
		})

		It("Should process false", func() {
			f := Form{
				Description: "test",
				Properties: []Property{
					{Name: "enabled", Description: "enable?", Type: BoolType},
				},
			}

			mock.EXPECT().AskOne(gomock.Any(), gomock.Any()).Return(nil)
			mockBoolResponse(mock, false)

			res, err := ProcessForm(f, nil, opts...)
			Expect(err).ToNot(HaveOccurred())
			Expect(res).To(Equal(map[string]any{"enabled": false}))
		})

		It("Should process bool with default", func() {
			f := Form{
				Description: "test",
				Properties: []Property{
					{Name: "flag", Description: "flag", Type: BoolType, Default: "true"},
				},
			}

			mock.EXPECT().AskOne(gomock.Any(), gomock.Any()).Return(nil)
			mockBoolResponse(mock, true)

			res, err := ProcessForm(f, nil, opts...)
			Expect(err).ToNot(HaveOccurred())
			Expect(res).To(Equal(map[string]any{"flag": true}))
		})

		It("Should fail with invalid bool default", func() {
			f := Form{
				Description: "test",
				Properties: []Property{
					{Name: "flag", Description: "flag", Type: BoolType, Default: "notabool"},
				},
			}

			mock.EXPECT().AskOne(gomock.Any(), gomock.Any()).Return(nil)

			_, err := ProcessForm(f, nil, opts...)
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("Error propagation", func() {
		It("Should propagate survey errors from string prompts", func() {
			f := Form{
				Description: "test",
				Properties: []Property{
					{Name: "x", Description: "x", Type: StringType},
				},
			}

			mock.EXPECT().AskOne(gomock.Any(), gomock.Any()).Return(nil)
			mock.EXPECT().AskOne(gomock.Any(), gomock.Any()).Return(fmt.Errorf("interrupted"))

			_, err := ProcessForm(f, nil, opts...)
			Expect(err).To(MatchError("interrupted"))
		})

		It("Should propagate survey errors from bool prompts", func() {
			f := Form{
				Description: "test",
				Properties: []Property{
					{Name: "b", Description: "b", Type: BoolType},
				},
			}

			mock.EXPECT().AskOne(gomock.Any(), gomock.Any()).Return(nil)
			mock.EXPECT().AskOne(gomock.Any(), gomock.Any()).Return(fmt.Errorf("canceled"))

			_, err := ProcessForm(f, nil, opts...)
			Expect(err).To(MatchError("canceled"))
		})

		It("Should propagate survey errors from int prompts", func() {
			f := Form{
				Description: "test",
				Properties: []Property{
					{Name: "n", Description: "n", Type: IntType},
				},
			}

			mock.EXPECT().AskOne(gomock.Any(), gomock.Any()).Return(nil)
			mock.EXPECT().AskOne(gomock.Any(), gomock.Any(), gomock.Any()).Return(fmt.Errorf("abort"))

			_, err := ProcessForm(f, nil, opts...)
			Expect(err).To(MatchError("abort"))
		})

		It("Should propagate survey errors from float prompts", func() {
			f := Form{
				Description: "test",
				Properties: []Property{
					{Name: "f", Description: "f", Type: FloatType},
				},
			}

			mock.EXPECT().AskOne(gomock.Any(), gomock.Any()).Return(nil)
			mock.EXPECT().AskOne(gomock.Any(), gomock.Any(), gomock.Any()).Return(fmt.Errorf("abort"))

			_, err := ProcessForm(f, nil, opts...)
			Expect(err).To(MatchError("abort"))
		})
	})
})
