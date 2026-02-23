// Copyright (c) 2023-2024, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package forms

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
)

var _ = Describe("Entry point functions", func() {
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

	validYAML := `
name: test
description: a test form
properties:
  - name: greeting
    description: greeting
    type: string
`

	Describe("ProcessBytes", func() {
		It("Should parse valid YAML and process the form", func() {
			gomock.InOrder(
				mock.EXPECT().AskOne(gomock.Any(), gomock.Any()).Return(nil),
				mockStringResponse(mock, "hello"),
			)

			res, err := ProcessBytes([]byte(validYAML), nil, opts...)
			Expect(err).ToNot(HaveOccurred())
			Expect(res).To(Equal(map[string]any{"greeting": "hello"}))
		})

		It("Should return error for invalid YAML", func() {
			_, err := ProcessBytes([]byte(":::bad yaml"), nil, opts...)
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("ProcessReader", func() {
		It("Should read and process valid YAML", func() {
			gomock.InOrder(
				mock.EXPECT().AskOne(gomock.Any(), gomock.Any()).Return(nil),
				mockStringResponse(mock, "world"),
			)

			res, err := ProcessReader(strings.NewReader(validYAML), nil, opts...)
			Expect(err).ToNot(HaveOccurred())
			Expect(res).To(Equal(map[string]any{"greeting": "world"}))
		})

		It("Should propagate reader errors", func() {
			_, err := ProcessReader(&errReader{}, nil, opts...)
			Expect(err).To(MatchError("read error"))
		})
	})

	Describe("ProcessFile", func() {
		It("Should read and process a YAML file", func() {
			gomock.InOrder(
				mock.EXPECT().AskOne(gomock.Any(), gomock.Any()).Return(nil),
				mockStringResponse(mock, "from file"),
			)

			tmp := filepath.Join(GinkgoT().TempDir(), "form.yaml")
			Expect(os.WriteFile(tmp, []byte(validYAML), 0644)).To(Succeed())

			res, err := ProcessFile(tmp, nil, opts...)
			Expect(err).ToNot(HaveOccurred())
			Expect(res).To(Equal(map[string]any{"greeting": "from file"}))
		})

		It("Should return error for non-existent file", func() {
			_, err := ProcessFile("/no/such/file.yaml", nil, opts...)
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("RenderedDescription", func() {
		It("Should render plain text", func() {
			p := &Property{Description: "hello world"}
			d, err := p.RenderedDescription(nil)
			Expect(err).ToNot(HaveOccurred())
			Expect(d).To(Equal("hello world"))
		})

		It("Should render templates with env", func() {
			p := &Property{Description: "Hello {{ .Name }}"}
			d, err := p.RenderedDescription(map[string]any{"Name": "Bob"})
			Expect(err).ToNot(HaveOccurred())
			Expect(d).To(Equal("Hello Bob"))
		})

		It("Should support sprig functions", func() {
			p := &Property{Description: `{{ "hello" | upper }}`}
			d, err := p.RenderedDescription(nil)
			Expect(err).ToNot(HaveOccurred())
			Expect(d).To(Equal("HELLO"))
		})

		It("Should return error for invalid template", func() {
			p := &Property{Description: "{{ .Bad | nosuchfunc }}"}
			_, err := p.RenderedDescription(nil)
			Expect(err).To(HaveOccurred())
		})
	})
})

// errReader is an io.Reader that always returns an error
type errReader struct{}

func (e *errReader) Read(p []byte) (int, error) {
	return 0, fmt.Errorf("read error")
}
