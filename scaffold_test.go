// Copyright (c) 2023-2024, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package scaffold

import (
	"os"
	"path/filepath"
	"testing"
	"text/template"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestScaffold(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Scaffold")
}

var _ = Describe("Scaffold", func() {
	var targetDir string

	BeforeEach(func() {
		targetDir = filepath.Join(GinkgoT().TempDir(), "target")
	})

	absTestdata := func(sub string) string {
		abs, err := filepath.Abs(filepath.Join("testdata", sub))
		Expect(err).ToNot(HaveOccurred())
		return abs
	}

	Describe("New", func() {
		DescribeTable("Validation errors",
			func(cfg Config, errMatch string) {
				_, err := New(cfg, nil)
				Expect(err).To(MatchError(ContainSubstring(errMatch)))
			},
			Entry("no target",
				Config{Source: map[string]any{"f": "c"}},
				"target is required"),
			Entry("no source",
				Config{TargetDirectory: "/tmp/scaffold-validation-test"},
				"no sources provided"),
			Entry("missing source directory",
				Config{TargetDirectory: "/tmp/scaffold-validation-test", SourceDirectory: "/no/such/directory"},
				"cannot read source directory"),
		)

		It("Should require target directory to not exist", func() {
			Expect(os.MkdirAll(targetDir, 0700)).To(Succeed())

			_, err := New(Config{
				TargetDirectory: targetDir,
				Source:          map[string]any{"f": "c"},
			}, nil)
			Expect(err).To(MatchError("target directory exist"))
		})

		It("Should create a valid scaffold with in-memory source", func() {
			s, err := New(Config{
				TargetDirectory: targetDir,
				Source:          map[string]any{"f": "c"},
			}, nil)
			Expect(err).ToNot(HaveOccurred())
			Expect(s).ToNot(BeNil())
			Expect(s.cfg.TargetDirectory).To(Equal(targetDir))
		})

		It("Should create a valid scaffold with source directory", func() {
			s, err := New(Config{
				TargetDirectory: targetDir,
				SourceDirectory: absTestdata("simple"),
			}, nil)
			Expect(err).ToNot(HaveOccurred())
			Expect(s).ToNot(BeNil())
		})

		It("Should resolve the target to an absolute path", func() {
			td := filepath.Join(GinkgoT().TempDir(), "abs-test")

			s, err := New(Config{
				TargetDirectory: td,
				Source:          map[string]any{"f": "c"},
			}, nil)
			Expect(err).ToNot(HaveOccurred())
			Expect(filepath.IsAbs(s.cfg.TargetDirectory)).To(BeTrue())
		})
	})

	Describe("RenderString", func() {
		DescribeTable("Rendering",
			func(cfg Config, funcs template.FuncMap, tmpl string, data any, expected string) {
				cfg.TargetDirectory = targetDir
				if cfg.Source == nil {
					cfg.Source = map[string]any{"f": "c"}
				}
				s, err := New(cfg, funcs)
				Expect(err).ToNot(HaveOccurred())

				result, err := s.RenderString(tmpl, data)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal(expected))
			},
			Entry("basic template",
				Config{}, template.FuncMap{},
				"Hello {{ .Name }}", map[string]any{"Name": "World"},
				"Hello World"),
			Entry("sprig upper function",
				Config{}, template.FuncMap{},
				`{{ "hello" | upper }}`, nil,
				"HELLO"),
			Entry("custom delimiters",
				Config{CustomLeftDelimiter: "<<", CustomRightDelimiter: ">>"},
				template.FuncMap{},
				"Hello << .Name >>", map[string]any{"Name": "World"},
				"Hello World"),
			Entry("custom functions",
				Config{},
				template.FuncMap{"greet": func(name string) string { return "hi " + name }},
				`{{ greet "bob" }}`, nil,
				"hi bob"),
		)

		It("Should return errors for invalid templates", func() {
			s, err := New(Config{
				TargetDirectory: targetDir,
				Source:          map[string]any{"f": "c"},
			}, template.FuncMap{})
			Expect(err).ToNot(HaveOccurred())

			_, err = s.RenderString("{{ .Invalid | nosuchfunc }}", nil)
			Expect(err).To(HaveOccurred())
		})

		It("Should handle SkipEmpty", func() {
			s, err := New(Config{
				TargetDirectory: targetDir,
				Source:          map[string]any{"f": "c"},
				SkipEmpty:       true,
			}, template.FuncMap{})
			Expect(err).ToNot(HaveOccurred())

			_, err = s.RenderString("{{ if false }}x{{ end }}", nil)
			Expect(err).To(MatchError(errSkippedEmpty))
		})
	})

	Describe("Render", func() {
		Context("With source directory", func() {
			It("Should render simple templates", func() {
				s, err := New(Config{
					TargetDirectory: targetDir,
					SourceDirectory: absTestdata("simple"),
				}, template.FuncMap{})
				Expect(err).ToNot(HaveOccurred())

				Expect(s.Render(map[string]any{"Name": "World"})).To(Succeed())

				content, err := os.ReadFile(filepath.Join(targetDir, "hello.txt"))
				Expect(err).ToNot(HaveOccurred())
				Expect(string(content)).To(Equal("Hello World"))
			})

			It("Should render nested directory structures", func() {
				s, err := New(Config{
					TargetDirectory: targetDir,
					SourceDirectory: absTestdata("nested"),
				}, template.FuncMap{})
				Expect(err).ToNot(HaveOccurred())

				Expect(s.Render(map[string]any{"Name": "Top", "Value": "Deep"})).To(Succeed())

				content, err := os.ReadFile(filepath.Join(targetDir, "top.txt"))
				Expect(err).ToNot(HaveOccurred())
				Expect(string(content)).To(Equal("Top: Top"))

				content, err = os.ReadFile(filepath.Join(targetDir, "sub", "deep.txt"))
				Expect(err).ToNot(HaveOccurred())
				Expect(string(content)).To(Equal("Deep: Deep"))
			})

			It("Should skip _partials directories", func() {
				s, err := New(Config{
					TargetDirectory: targetDir,
					SourceDirectory: absTestdata("with_partials"),
				}, template.FuncMap{})
				Expect(err).ToNot(HaveOccurred())

				Expect(s.Render(map[string]any{"Name": "Test"})).To(Succeed())

				content, err := os.ReadFile(filepath.Join(targetDir, "main.txt"))
				Expect(err).ToNot(HaveOccurred())
				Expect(string(content)).To(Equal("Main: Test"))

				_, err = os.Stat(filepath.Join(targetDir, "_partials"))
				Expect(os.IsNotExist(err)).To(BeTrue())
			})

			It("Should render with custom delimiters", func() {
				s, err := New(Config{
					TargetDirectory:      targetDir,
					SourceDirectory:      absTestdata("custom_delims"),
					CustomLeftDelimiter:  "<<",
					CustomRightDelimiter: ">>",
				}, template.FuncMap{})
				Expect(err).ToNot(HaveOccurred())

				Expect(s.Render(map[string]any{"Name": "World"})).To(Succeed())

				content, err := os.ReadFile(filepath.Join(targetDir, "greeting.txt"))
				Expect(err).ToNot(HaveOccurred())
				Expect(string(content)).To(Equal("Hello World"))
			})

			It("Should skip empty files when SkipEmpty is set", func() {
				s, err := New(Config{
					TargetDirectory: targetDir,
					SourceDirectory: absTestdata("with_empty"),
					SkipEmpty:       true,
				}, template.FuncMap{})
				Expect(err).ToNot(HaveOccurred())

				Expect(s.Render(map[string]any{"Name": "Test", "Show": false})).To(Succeed())

				_, err = os.Stat(filepath.Join(targetDir, "maybe.txt"))
				Expect(os.IsNotExist(err)).To(BeTrue())

				content, err := os.ReadFile(filepath.Join(targetDir, "present.txt"))
				Expect(err).ToNot(HaveOccurred())
				Expect(string(content)).To(Equal("always Test"))
			})

			It("Should not skip empty files when SkipEmpty is not set", func() {
				s, err := New(Config{
					TargetDirectory: targetDir,
					SourceDirectory: absTestdata("with_empty"),
				}, template.FuncMap{})
				Expect(err).ToNot(HaveOccurred())

				Expect(s.Render(map[string]any{"Name": "Test", "Show": false})).To(Succeed())

				content, err := os.ReadFile(filepath.Join(targetDir, "maybe.txt"))
				Expect(err).ToNot(HaveOccurred())
				Expect(string(content)).To(Equal(""))
			})

			It("Should support the render template function", func() {
				s, err := New(Config{
					TargetDirectory: targetDir,
					SourceDirectory: absTestdata("with_render"),
				}, template.FuncMap{})
				Expect(err).ToNot(HaveOccurred())

				Expect(s.Render(map[string]any{"Name": "Rendered"})).To(Succeed())

				content, err := os.ReadFile(filepath.Join(targetDir, "output.txt"))
				Expect(err).ToNot(HaveOccurred())
				Expect(string(content)).To(Equal("partial: Rendered"))
			})

			It("Should support the write template function", func() {
				s, err := New(Config{
					TargetDirectory: targetDir,
					SourceDirectory: absTestdata("with_write"),
				}, template.FuncMap{})
				Expect(err).ToNot(HaveOccurred())

				Expect(s.Render(nil)).To(Succeed())

				content, err := os.ReadFile(filepath.Join(targetDir, "main.txt"))
				Expect(err).ToNot(HaveOccurred())
				Expect(string(content)).To(Equal("main"))

				content, err = os.ReadFile(filepath.Join(targetDir, "extra.txt"))
				Expect(err).ToNot(HaveOccurred())
				Expect(string(content)).To(Equal("extra content"))
			})
		})

		Context("With in-memory source", func() {
			It("Should render simple templates", func() {
				s, err := New(Config{
					TargetDirectory: targetDir,
					Source: map[string]any{
						"hello.txt": "Hello {{ .Name }}",
					},
				}, template.FuncMap{})
				Expect(err).ToNot(HaveOccurred())

				Expect(s.Render(map[string]any{"Name": "Memory"})).To(Succeed())

				content, err := os.ReadFile(filepath.Join(targetDir, "hello.txt"))
				Expect(err).ToNot(HaveOccurred())
				Expect(string(content)).To(Equal("Hello Memory"))
			})

			It("Should render nested directory structures", func() {
				s, err := New(Config{
					TargetDirectory: targetDir,
					Source: map[string]any{
						"root.txt": "Root: {{ .Name }}",
						"sub": map[string]any{
							"child.txt": "Child: {{ .Value }}",
						},
					},
				}, template.FuncMap{})
				Expect(err).ToNot(HaveOccurred())

				Expect(s.Render(map[string]any{"Name": "Top", "Value": "Nested"})).To(Succeed())

				content, err := os.ReadFile(filepath.Join(targetDir, "root.txt"))
				Expect(err).ToNot(HaveOccurred())
				Expect(string(content)).To(Equal("Root: Top"))

				content, err = os.ReadFile(filepath.Join(targetDir, "sub", "child.txt"))
				Expect(err).ToNot(HaveOccurred())
				Expect(string(content)).To(Equal("Child: Nested"))
			})

			DescribeTable("Invalid source entries",
				func(source map[string]any, errMatch string) {
					s, err := New(Config{
						TargetDirectory: targetDir,
						Source:          source,
					}, template.FuncMap{})
					Expect(err).ToNot(HaveOccurred())

					err = s.Render(nil)
					Expect(err).To(MatchError(ContainSubstring(errMatch)))
				},
				Entry("filename with ..",
					map[string]any{"../escape.txt": "bad"},
					"invalid file name"),
				Entry("filename with forward slash",
					map[string]any{"sub/file.txt": "bad"},
					"invalid file name"),
				Entry("filename with backslash",
					map[string]any{"sub\\file.txt": "bad"},
					"invalid file name"),
				Entry("non-string non-map value",
					map[string]any{"bad.txt": 12345},
					"invalid source entry"),
			)

			It("Should clean up temporary source directory on success", func() {
				s, err := New(Config{
					TargetDirectory: targetDir,
					Source: map[string]any{
						"test.txt": "content",
					},
				}, template.FuncMap{})
				Expect(err).ToNot(HaveOccurred())

				Expect(s.Render(nil)).To(Succeed())
				Expect(s.workingSource).To(Equal(""))
			})
		})

		Context("With post-processing", func() {
			It("Should post-process matching files", func() {
				s, err := New(Config{
					TargetDirectory: targetDir,
					SourceDirectory: absTestdata("simple"),
					Post: []map[string]string{
						{"*.txt": "chmod 600 {}"},
					},
				}, template.FuncMap{})
				Expect(err).ToNot(HaveOccurred())

				Expect(s.Render(map[string]any{"Name": "World"})).To(Succeed())

				info, err := os.Stat(filepath.Join(targetDir, "hello.txt"))
				Expect(err).ToNot(HaveOccurred())
				Expect(info.Mode().Perm()).To(Equal(os.FileMode(0600)))
			})

			It("Should not post-process non-matching files", func() {
				s, err := New(Config{
					TargetDirectory: targetDir,
					SourceDirectory: absTestdata("simple"),
					Post: []map[string]string{
						{"*.go": "chmod 600 {}"},
					},
				}, template.FuncMap{})
				Expect(err).ToNot(HaveOccurred())

				Expect(s.Render(map[string]any{"Name": "World"})).To(Succeed())

				info, err := os.Stat(filepath.Join(targetDir, "hello.txt"))
				Expect(err).ToNot(HaveOccurred())
				Expect(info.Mode().Perm()).To(Equal(os.FileMode(0755)))
			})

			It("Should append file as last argument when {} is not used", func() {
				s, err := New(Config{
					TargetDirectory: targetDir,
					SourceDirectory: absTestdata("simple"),
					Post: []map[string]string{
						{"*.txt": "chmod 600"},
					},
				}, template.FuncMap{})
				Expect(err).ToNot(HaveOccurred())

				Expect(s.Render(map[string]any{"Name": "World"})).To(Succeed())

				info, err := os.Stat(filepath.Join(targetDir, "hello.txt"))
				Expect(err).ToNot(HaveOccurred())
				Expect(info.Mode().Perm()).To(Equal(os.FileMode(0600)))
			})

			It("Should fail on invalid post-processing commands", func() {
				s, err := New(Config{
					TargetDirectory: targetDir,
					SourceDirectory: absTestdata("simple"),
					Post: []map[string]string{
						{"*.txt": "/no/such/command"},
					},
				}, template.FuncMap{})
				Expect(err).ToNot(HaveOccurred())

				err = s.Render(map[string]any{"Name": "World"})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to post process"))
			})
		})

		It("Should create the target directory", func() {
			s, err := New(Config{
				TargetDirectory: targetDir,
				Source: map[string]any{
					"test.txt": "hello",
				},
			}, template.FuncMap{})
			Expect(err).ToNot(HaveOccurred())

			Expect(s.Render(nil)).To(Succeed())

			info, err := os.Stat(targetDir)
			Expect(err).ToNot(HaveOccurred())
			Expect(info.IsDir()).To(BeTrue())
		})

		It("Should render without funcs when nil is passed", func() {
			s, err := New(Config{
				TargetDirectory: targetDir,
				Source: map[string]any{
					"plain.txt": "no templates here",
				},
			}, nil)
			Expect(err).ToNot(HaveOccurred())

			Expect(s.Render(nil)).To(Succeed())

			content, err := os.ReadFile(filepath.Join(targetDir, "plain.txt"))
			Expect(err).ToNot(HaveOccurred())
			Expect(string(content)).To(Equal("no templates here"))
		})
	})

	Describe("Logger", func() {
		It("Should set the logger", func() {
			s, err := New(Config{
				TargetDirectory: targetDir,
				Source:          map[string]any{"f": "c"},
			}, nil)
			Expect(err).ToNot(HaveOccurred())

			Expect(s.log).To(BeNil())
			s.Logger(&testLogger{})
			Expect(s.log).ToNot(BeNil())
		})
	})

	Describe("saveFile", func() {
		It("Should reject files outside the target directory", func() {
			s, err := New(Config{
				TargetDirectory: targetDir,
				Source:          map[string]any{"f": "c"},
			}, nil)
			Expect(err).ToNot(HaveOccurred())

			Expect(os.MkdirAll(targetDir, 0700)).To(Succeed())

			err = s.saveFile("/tmp/outside.txt", "content")
			Expect(err).To(MatchError(ContainSubstring("is not in target directory")))
		})
	})
})

type testLogger struct{}

func (t *testLogger) Debugf(format string, v ...any) {}
func (t *testLogger) Infof(format string, v ...any)  {}
