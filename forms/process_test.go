// Copyright (c) 2023-2024, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package forms

import (
	"io"

	"github.com/AlecAivazis/survey/v2"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
)

// testOpts returns processOption slice wired to the given mock
func testOpts(mock *Mocksurveyor) []processOption {
	return []processOption{
		withSurveyor(mock),
		withIsTerminal(func() bool { return true }),
		withOutput(io.Discard),
	}
}

// mockStringResponse matches an AskOne call with NO validator opts (2 args)
func mockStringResponse(mock *Mocksurveyor, answer string) *MocksurveyorAskOneCall {
	return mock.EXPECT().AskOne(gomock.Any(), gomock.Any()).
		DoAndReturn(func(p survey.Prompt, resp any, opts ...survey.AskOpt) error {
			if ptr, ok := resp.(*string); ok {
				*ptr = answer
			}
			return nil
		})
}

// mockStringResponseV matches an AskOne call WITH validator opts (3+ args)
func mockStringResponseV(mock *Mocksurveyor, answer string) *MocksurveyorAskOneCall {
	return mock.EXPECT().AskOne(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(p survey.Prompt, resp any, opts ...survey.AskOpt) error {
			if ptr, ok := resp.(*string); ok {
				*ptr = answer
			}
			return nil
		})
}

// mockBoolResponse matches an AskOne call with NO validator opts (2 args)
func mockBoolResponse(mock *Mocksurveyor, answer bool) *MocksurveyorAskOneCall {
	return mock.EXPECT().AskOne(gomock.Any(), gomock.Any()).
		DoAndReturn(func(p survey.Prompt, resp any, opts ...survey.AskOpt) error {
			if ptr, ok := resp.(*bool); ok {
				*ptr = answer
			}
			return nil
		})
}

var _ = Describe("ProcessForm", func() {
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

	It("Should fail with no properties", func() {
		f := Form{Description: "empty", Properties: nil}
		_, err := ProcessForm(f, nil, opts...)
		Expect(err).To(MatchError("no properties defined"))
	})

	It("Should fail when not a terminal", func() {
		f := Form{Description: "test", Properties: []Property{{Name: "x", Type: StringType}}}
		notTermOpts := []processOption{
			withSurveyor(mock),
			withIsTerminal(func() bool { return false }),
			withOutput(io.Discard),
		}
		_, err := ProcessForm(f, nil, notTermOpts...)
		Expect(err).To(MatchError("can only process forms on a valid terminal"))
	})

	It("Should process a single required string property", func() {
		f := Form{
			Description: "test form",
			Properties: []Property{
				{Name: "name", Description: "enter name", Type: StringType, Required: true},
			},
		}

		// "Press enter to start" prompt
		mock.EXPECT().AskOne(gomock.Any(), gomock.Any()).Return(nil)
		// string input (required -> has MinLength validator -> 3 args)
		mockStringResponseV(mock, "hello")

		res, err := ProcessForm(f, nil, opts...)
		Expect(err).ToNot(HaveOccurred())
		Expect(res).To(Equal(map[string]any{"name": "hello"}))
	})

	It("Should process an optional empty string as absent", func() {
		f := Form{
			Description: "test",
			Properties: []Property{
				{Name: "opt", Description: "optional", Type: StringType, IfEmpty: AbsentIfEmpty},
			},
		}

		mock.EXPECT().AskOne(gomock.Any(), gomock.Any()).Return(nil) // press enter
		mockStringResponse(mock, "")

		res, err := ProcessForm(f, nil, opts...)
		Expect(err).ToNot(HaveOccurred())
		Expect(res).To(Equal(map[string]any{}))
	})

	It("Should process an optional string with IfEmpty=object", func() {
		f := Form{
			Description: "test",
			Properties: []Property{
				{Name: "opt", Description: "optional", Type: StringType, IfEmpty: ObjectIfEmpty},
			},
		}

		mock.EXPECT().AskOne(gomock.Any(), gomock.Any()).Return(nil)
		mockStringResponse(mock, "")

		res, err := ProcessForm(f, nil, opts...)
		Expect(err).ToNot(HaveOccurred())
		Expect(res).To(Equal(map[string]any{"opt": map[string]any{}}))
	})

	It("Should process an optional string with IfEmpty=array", func() {
		f := Form{
			Description: "test",
			Properties: []Property{
				{Name: "opt", Description: "optional", Type: StringType, IfEmpty: ArrayIfEmpty},
			},
		}

		mock.EXPECT().AskOne(gomock.Any(), gomock.Any()).Return(nil)
		mockStringResponse(mock, "")

		res, err := ProcessForm(f, nil, opts...)
		Expect(err).ToNot(HaveOccurred())
		Expect(res).To(Equal(map[string]any{"opt": []any{}}))
	})

	It("Should process a string with default", func() {
		f := Form{
			Description: "test",
			Properties: []Property{
				{Name: "color", Description: "pick", Type: StringType, Default: "blue"},
			},
		}

		mock.EXPECT().AskOne(gomock.Any(), gomock.Any()).Return(nil)
		mockStringResponse(mock, "blue")

		res, err := ProcessForm(f, nil, opts...)
		Expect(err).ToNot(HaveOccurred())
		Expect(res).To(Equal(map[string]any{"color": "blue"}))
	})

	It("Should process a string enum property", func() {
		f := Form{
			Description: "test",
			Properties: []Property{
				{Name: "size", Description: "pick size", Type: StringType, Enum: []string{"s", "m", "l"}},
			},
		}

		mock.EXPECT().AskOne(gomock.Any(), gomock.Any()).Return(nil) // press enter
		// enum calls AskOne with opts (validators)
		mockStringResponse(mock, "m")

		res, err := ProcessForm(f, nil, opts...)
		Expect(err).ToNot(HaveOccurred())
		Expect(res).To(Equal(map[string]any{"size": "m"}))
	})

	It("Should process a password property", func() {
		f := Form{
			Description: "test",
			Properties: []Property{
				{Name: "secret", Description: "enter secret", Type: PasswordType},
			},
		}

		mock.EXPECT().AskOne(gomock.Any(), gomock.Any()).Return(nil)
		mockStringResponse(mock, "s3cret")

		res, err := ProcessForm(f, nil, opts...)
		Expect(err).ToNot(HaveOccurred())
		Expect(res).To(Equal(map[string]any{"secret": "s3cret"}))
	})
})
