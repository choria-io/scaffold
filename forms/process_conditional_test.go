// Copyright (c) 2023-2024, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package forms

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
)

var _ = Describe("ProcessForm conditionals", func() {
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

	It("Should skip properties when conditional is false", func() {
		f := Form{
			Description: "test",
			Properties: []Property{
				{Name: "mode", Description: "mode", Type: StringType},
				{Name: "advanced", Description: "advanced", Type: StringType, ConditionalExpression: `input.mode == "expert"`},
			},
		}

		gomock.InOrder(
			mock.EXPECT().AskOne(gomock.Any(), gomock.Any()).Return(nil),
			// mode
			mockStringResponse(mock, "basic"),
			// advanced should be skipped since mode != "expert"
		)

		res, err := ProcessForm(f, nil, opts...)
		Expect(err).ToNot(HaveOccurred())
		Expect(res).To(Equal(map[string]any{"mode": "basic"}))
	})

	It("Should include properties when conditional is true", func() {
		f := Form{
			Description: "test",
			Properties: []Property{
				{Name: "mode", Description: "mode", Type: StringType},
				{Name: "advanced", Description: "advanced", Type: StringType, ConditionalExpression: `input.mode == "expert"`},
			},
		}

		gomock.InOrder(
			mock.EXPECT().AskOne(gomock.Any(), gomock.Any()).Return(nil),
			mockStringResponse(mock, "expert"),
			mockStringResponse(mock, "extra-setting"),
		)

		res, err := ProcessForm(f, nil, opts...)
		Expect(err).ToNot(HaveOccurred())
		Expect(res).To(Equal(map[string]any{"mode": "expert", "advanced": "extra-setting"}))
	})

	It("Should reference env in conditionals", func() {
		f := Form{
			Description: "test",
			Properties: []Property{
				{Name: "debug", Description: "debug", Type: BoolType, ConditionalExpression: `Verbose == true`},
			},
		}

		gomock.InOrder(
			mock.EXPECT().AskOne(gomock.Any(), gomock.Any()).Return(nil),
			mockBoolResponse(mock, true),
		)

		env := map[string]any{"Verbose": true}
		res, err := ProcessForm(f, env, opts...)
		Expect(err).ToNot(HaveOccurred())
		Expect(res).To(Equal(map[string]any{"debug": true}))
	})
})
