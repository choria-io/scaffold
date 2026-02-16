// Copyright (c) 2023-2024, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package scaffold

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"text/template"

	"github.com/CloudyKit/jet/v6"
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
			Expect(err).To(MatchError("target directory exists"))
		})

		It("Should allow existing target directory when MergeTargetDirectory is set", func() {
			Expect(os.MkdirAll(targetDir, 0700)).To(Succeed())

			s, err := New(Config{
				TargetDirectory:      targetDir,
				MergeTargetDirectory: true,
				Source:               map[string]any{"f": "c"},
			}, nil)
			Expect(err).ToNot(HaveOccurred())
			Expect(s).ToNot(BeNil())
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

	Describe("NewJet", func() {
		DescribeTable("Validation errors",
			func(cfg Config, errMatch string) {
				_, err := NewJet(cfg, nil)
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

			_, err := NewJet(Config{
				TargetDirectory: targetDir,
				Source:          map[string]any{"f": "c"},
			}, nil)
			Expect(err).To(MatchError("target directory exists"))
		})

		It("Should allow existing target directory when MergeTargetDirectory is set", func() {
			Expect(os.MkdirAll(targetDir, 0700)).To(Succeed())

			s, err := NewJet(Config{
				TargetDirectory:      targetDir,
				MergeTargetDirectory: true,
				Source:               map[string]any{"f": "c"},
			}, nil)
			Expect(err).ToNot(HaveOccurred())
			Expect(s).ToNot(BeNil())
			Expect(s.engine).To(Equal(engineJet))
		})

		It("Should create a valid scaffold with in-memory source", func() {
			s, err := NewJet(Config{
				TargetDirectory: targetDir,
				Source:          map[string]any{"f": "c"},
			}, nil)
			Expect(err).ToNot(HaveOccurred())
			Expect(s).ToNot(BeNil())
			Expect(s.cfg.TargetDirectory).To(Equal(targetDir))
			Expect(s.engine).To(Equal(engineJet))
		})

		It("Should create a valid scaffold with source directory", func() {
			s, err := NewJet(Config{
				TargetDirectory: targetDir,
				SourceDirectory: absTestdata("simple"),
			}, nil)
			Expect(err).ToNot(HaveOccurred())
			Expect(s).ToNot(BeNil())
			Expect(s.engine).To(Equal(engineJet))
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

		Context("With Jet engine", func() {
			It("Should render a basic template", func() {
				s, err := NewJet(Config{
					TargetDirectory: targetDir,
					Source:          map[string]any{"f": "c"},
				}, map[string]jet.Func{})
				Expect(err).ToNot(HaveOccurred())

				result, err := s.RenderString("Hello {{ .Name }}", map[string]any{"Name": "World"})
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal("Hello World"))
			})

			It("Should support custom Jet functions", func() {
				funcs := map[string]jet.Func{
					"greet": func(args jet.Arguments) reflect.Value {
						args.RequireNumOfArguments("greet", 1, 1)
						var name string
						err := args.ParseInto(&name)
						if err != nil {
							args.Panicf("greet: %v", err)
						}
						return reflect.ValueOf("hi " + name)
					},
				}

				s, err := NewJet(Config{
					TargetDirectory: targetDir,
					Source:          map[string]any{"f": "c"},
				}, funcs)
				Expect(err).ToNot(HaveOccurred())

				result, err := s.RenderString(`{{ greet("bob") }}`, nil)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal("hi bob"))
			})

			It("Should support custom delimiters", func() {
				s, err := NewJet(Config{
					TargetDirectory:      targetDir,
					Source:               map[string]any{"f": "c"},
					CustomLeftDelimiter:  "<<",
					CustomRightDelimiter: ">>",
				}, map[string]jet.Func{})
				Expect(err).ToNot(HaveOccurred())

				result, err := s.RenderString("Hello << .Name >>", map[string]any{"Name": "World"})
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal("Hello World"))
			})

			It("Should handle SkipEmpty", func() {
				s, err := NewJet(Config{
					TargetDirectory: targetDir,
					Source:          map[string]any{"f": "c"},
					SkipEmpty:       true,
				}, map[string]jet.Func{})
				Expect(err).ToNot(HaveOccurred())

				_, err = s.RenderString("{{ if false }}x{{ end }}", nil)
				Expect(err).To(MatchError(errSkippedEmpty))
			})

			It("Should return errors for invalid templates", func() {
				s, err := NewJet(Config{
					TargetDirectory: targetDir,
					Source:          map[string]any{"f": "c"},
				}, map[string]jet.Func{})
				Expect(err).ToNot(HaveOccurred())

				_, err = s.RenderString("{{ nosuchfunc() }}", nil)
				Expect(err).To(HaveOccurred())
			})
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
				Expect(info.Mode().Perm()).To(Equal(os.FileMode(0644)))
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

		Context("With Jet engine", func() {
			It("Should render simple templates", func() {
				s, err := NewJet(Config{
					TargetDirectory: targetDir,
					SourceDirectory: absTestdata("simple"),
				}, map[string]jet.Func{})
				Expect(err).ToNot(HaveOccurred())

				Expect(s.Render(map[string]any{"Name": "World"})).To(Succeed())

				content, err := os.ReadFile(filepath.Join(targetDir, "hello.txt"))
				Expect(err).ToNot(HaveOccurred())
				Expect(string(content)).To(Equal("Hello World"))
			})

			It("Should render nested directory structures", func() {
				s, err := NewJet(Config{
					TargetDirectory: targetDir,
					SourceDirectory: absTestdata("nested"),
				}, map[string]jet.Func{})
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
				s, err := NewJet(Config{
					TargetDirectory: targetDir,
					SourceDirectory: absTestdata("with_partials"),
				}, map[string]jet.Func{})
				Expect(err).ToNot(HaveOccurred())

				Expect(s.Render(map[string]any{"Name": "Test"})).To(Succeed())

				content, err := os.ReadFile(filepath.Join(targetDir, "main.txt"))
				Expect(err).ToNot(HaveOccurred())
				Expect(string(content)).To(Equal("Main: Test"))

				_, err = os.Stat(filepath.Join(targetDir, "_partials"))
				Expect(os.IsNotExist(err)).To(BeTrue())
			})

			It("Should render with custom delimiters", func() {
				s, err := NewJet(Config{
					TargetDirectory:      targetDir,
					SourceDirectory:      absTestdata("custom_delims"),
					CustomLeftDelimiter:  "<<",
					CustomRightDelimiter: ">>",
				}, map[string]jet.Func{})
				Expect(err).ToNot(HaveOccurred())

				Expect(s.Render(map[string]any{"Name": "World"})).To(Succeed())

				content, err := os.ReadFile(filepath.Join(targetDir, "greeting.txt"))
				Expect(err).ToNot(HaveOccurred())
				Expect(string(content)).To(Equal("Hello World"))
			})

			It("Should skip empty files when SkipEmpty is set", func() {
				s, err := NewJet(Config{
					TargetDirectory: targetDir,
					SourceDirectory: absTestdata("with_empty"),
					SkipEmpty:       true,
				}, map[string]jet.Func{})
				Expect(err).ToNot(HaveOccurred())

				Expect(s.Render(map[string]any{"Name": "Test", "Show": false})).To(Succeed())

				_, err = os.Stat(filepath.Join(targetDir, "maybe.txt"))
				Expect(os.IsNotExist(err)).To(BeTrue())

				content, err := os.ReadFile(filepath.Join(targetDir, "present.txt"))
				Expect(err).ToNot(HaveOccurred())
				Expect(string(content)).To(Equal("always Test"))
			})

			It("Should support the render template function", func() {
				s, err := NewJet(Config{
					TargetDirectory: targetDir,
					SourceDirectory: absTestdata("jet_with_render"),
				}, map[string]jet.Func{})
				Expect(err).ToNot(HaveOccurred())

				Expect(s.Render(map[string]any{"Name": "Rendered"})).To(Succeed())

				content, err := os.ReadFile(filepath.Join(targetDir, "output.txt"))
				Expect(err).ToNot(HaveOccurred())
				Expect(string(content)).To(Equal("partial: Rendered"))
			})

			It("Should support the write template function", func() {
				s, err := NewJet(Config{
					TargetDirectory: targetDir,
					SourceDirectory: absTestdata("jet_with_write"),
				}, map[string]jet.Func{})
				Expect(err).ToNot(HaveOccurred())

				Expect(s.Render(nil)).To(Succeed())

				content, err := os.ReadFile(filepath.Join(targetDir, "main.txt"))
				Expect(err).ToNot(HaveOccurred())
				Expect(string(content)).To(Equal("main"))

				content, err = os.ReadFile(filepath.Join(targetDir, "extra.txt"))
				Expect(err).ToNot(HaveOccurred())
				Expect(string(content)).To(Equal("extra content"))
			})

			It("Should render with in-memory source", func() {
				s, err := NewJet(Config{
					TargetDirectory: targetDir,
					Source: map[string]any{
						"hello.txt": "Hello {{ .Name }}",
					},
				}, map[string]jet.Func{})
				Expect(err).ToNot(HaveOccurred())

				Expect(s.Render(map[string]any{"Name": "Memory"})).To(Succeed())

				content, err := os.ReadFile(filepath.Join(targetDir, "hello.txt"))
				Expect(err).ToNot(HaveOccurred())
				Expect(string(content)).To(Equal("Hello Memory"))
			})

			It("Should post-process matching files", func() {
				s, err := NewJet(Config{
					TargetDirectory: targetDir,
					SourceDirectory: absTestdata("simple"),
					Post: []map[string]string{
						{"*.txt": "chmod 600 {}"},
					},
				}, map[string]jet.Func{})
				Expect(err).ToNot(HaveOccurred())

				Expect(s.Render(map[string]any{"Name": "World"})).To(Succeed())

				info, err := os.Stat(filepath.Join(targetDir, "hello.txt"))
				Expect(err).ToNot(HaveOccurred())
				Expect(info.Mode().Perm()).To(Equal(os.FileMode(0600)))
			})

			It("Should render without funcs when nil is passed", func() {
				s, err := NewJet(Config{
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

		Context("With MergeTargetDirectory", func() {
			It("Should render into an existing directory", func() {
				Expect(os.MkdirAll(targetDir, 0700)).To(Succeed())

				s, err := New(Config{
					TargetDirectory:      targetDir,
					MergeTargetDirectory: true,
					Source: map[string]any{
						"hello.txt": "Hello {{ .Name }}",
					},
				}, template.FuncMap{})
				Expect(err).ToNot(HaveOccurred())

				Expect(s.Render(map[string]any{"Name": "World"})).To(Succeed())

				content, err := os.ReadFile(filepath.Join(targetDir, "hello.txt"))
				Expect(err).ToNot(HaveOccurred())
				Expect(string(content)).To(Equal("Hello World"))
			})

			It("Should preserve existing files in the target directory", func() {
				Expect(os.MkdirAll(targetDir, 0700)).To(Succeed())
				Expect(os.WriteFile(filepath.Join(targetDir, "existing.txt"), []byte("keep me"), 0644)).To(Succeed())

				s, err := New(Config{
					TargetDirectory:      targetDir,
					MergeTargetDirectory: true,
					Source: map[string]any{
						"new.txt": "new content",
					},
				}, template.FuncMap{})
				Expect(err).ToNot(HaveOccurred())

				Expect(s.Render(nil)).To(Succeed())

				content, err := os.ReadFile(filepath.Join(targetDir, "existing.txt"))
				Expect(err).ToNot(HaveOccurred())
				Expect(string(content)).To(Equal("keep me"))

				content, err = os.ReadFile(filepath.Join(targetDir, "new.txt"))
				Expect(err).ToNot(HaveOccurred())
				Expect(string(content)).To(Equal("new content"))
			})

			It("Should overwrite existing files with rendered content", func() {
				Expect(os.MkdirAll(targetDir, 0700)).To(Succeed())
				Expect(os.WriteFile(filepath.Join(targetDir, "hello.txt"), []byte("old content"), 0644)).To(Succeed())

				s, err := New(Config{
					TargetDirectory:      targetDir,
					MergeTargetDirectory: true,
					Source: map[string]any{
						"hello.txt": "Hello {{ .Name }}",
					},
				}, template.FuncMap{})
				Expect(err).ToNot(HaveOccurred())

				Expect(s.Render(map[string]any{"Name": "World"})).To(Succeed())

				content, err := os.ReadFile(filepath.Join(targetDir, "hello.txt"))
				Expect(err).ToNot(HaveOccurred())
				Expect(string(content)).To(Equal("Hello World"))
			})

			It("Should render with source directory into existing target", func() {
				Expect(os.MkdirAll(targetDir, 0700)).To(Succeed())
				Expect(os.WriteFile(filepath.Join(targetDir, "existing.txt"), []byte("keep me"), 0644)).To(Succeed())

				s, err := New(Config{
					TargetDirectory:      targetDir,
					MergeTargetDirectory: true,
					SourceDirectory:      absTestdata("simple"),
				}, template.FuncMap{})
				Expect(err).ToNot(HaveOccurred())

				Expect(s.Render(map[string]any{"Name": "World"})).To(Succeed())

				content, err := os.ReadFile(filepath.Join(targetDir, "existing.txt"))
				Expect(err).ToNot(HaveOccurred())
				Expect(string(content)).To(Equal("keep me"))

				content, err = os.ReadFile(filepath.Join(targetDir, "hello.txt"))
				Expect(err).ToNot(HaveOccurred())
				Expect(string(content)).To(Equal("Hello World"))
			})

			It("Should track changed files correctly", func() {
				Expect(os.MkdirAll(targetDir, 0700)).To(Succeed())
				Expect(os.WriteFile(filepath.Join(targetDir, "existing.txt"), []byte("keep me"), 0644)).To(Succeed())

				s, err := New(Config{
					TargetDirectory:      targetDir,
					MergeTargetDirectory: true,
					Source: map[string]any{
						"new.txt": "new content",
					},
				}, template.FuncMap{})
				Expect(err).ToNot(HaveOccurred())

				Expect(s.Render(nil)).To(Succeed())
				Expect(s.ChangedFiles()).To(ConsistOf("new.txt"))
			})

			Context("With Jet engine", func() {
				It("Should render into an existing directory", func() {
					Expect(os.MkdirAll(targetDir, 0700)).To(Succeed())

					s, err := NewJet(Config{
						TargetDirectory:      targetDir,
						MergeTargetDirectory: true,
						Source: map[string]any{
							"hello.txt": "Hello {{ .Name }}",
						},
					}, map[string]jet.Func{})
					Expect(err).ToNot(HaveOccurred())

					Expect(s.Render(map[string]any{"Name": "World"})).To(Succeed())

					content, err := os.ReadFile(filepath.Join(targetDir, "hello.txt"))
					Expect(err).ToNot(HaveOccurred())
					Expect(string(content)).To(Equal("Hello World"))
				})

				It("Should preserve existing files in the target directory", func() {
					Expect(os.MkdirAll(targetDir, 0700)).To(Succeed())
					Expect(os.WriteFile(filepath.Join(targetDir, "existing.txt"), []byte("keep me"), 0644)).To(Succeed())

					s, err := NewJet(Config{
						TargetDirectory:      targetDir,
						MergeTargetDirectory: true,
						Source: map[string]any{
							"new.txt": "new content",
						},
					}, map[string]jet.Func{})
					Expect(err).ToNot(HaveOccurred())

					Expect(s.Render(nil)).To(Succeed())

					content, err := os.ReadFile(filepath.Join(targetDir, "existing.txt"))
					Expect(err).ToNot(HaveOccurred())
					Expect(string(content)).To(Equal("keep me"))

					content, err = os.ReadFile(filepath.Join(targetDir, "new.txt"))
					Expect(err).ToNot(HaveOccurred())
					Expect(string(content)).To(Equal("new content"))
				})
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

	Describe("ChangedFiles", func() {
		It("Should be empty before any render", func() {
			s, err := New(Config{
				TargetDirectory: targetDir,
				Source:          map[string]any{"f": "c"},
			}, template.FuncMap{})
			Expect(err).ToNot(HaveOccurred())
			Expect(s.ChangedFiles()).To(BeNil())
		})

		It("Should track rendered files", func() {
			s, err := New(Config{
				TargetDirectory: targetDir,
				SourceDirectory: absTestdata("simple"),
			}, template.FuncMap{})
			Expect(err).ToNot(HaveOccurred())

			Expect(s.Render(map[string]any{"Name": "World"})).To(Succeed())
			Expect(s.ChangedFiles()).To(ConsistOf("hello.txt"))
		})

		It("Should use forward slashes for nested paths", func() {
			s, err := New(Config{
				TargetDirectory: targetDir,
				SourceDirectory: absTestdata("nested"),
			}, template.FuncMap{})
			Expect(err).ToNot(HaveOccurred())

			Expect(s.Render(map[string]any{"Name": "Top", "Value": "Deep"})).To(Succeed())
			Expect(s.ChangedFiles()).To(ConsistOf("top.txt", "sub/deep.txt"))
		})

		It("Should exclude skipped empty files", func() {
			s, err := New(Config{
				TargetDirectory: targetDir,
				SourceDirectory: absTestdata("with_empty"),
				SkipEmpty:       true,
			}, template.FuncMap{})
			Expect(err).ToNot(HaveOccurred())

			Expect(s.Render(map[string]any{"Name": "Test", "Show": false})).To(Succeed())
			Expect(s.ChangedFiles()).To(ConsistOf("present.txt"))
		})

		It("Should include all files when SkipEmpty is not set", func() {
			s, err := New(Config{
				TargetDirectory: targetDir,
				SourceDirectory: absTestdata("with_empty"),
			}, template.FuncMap{})
			Expect(err).ToNot(HaveOccurred())

			Expect(s.Render(map[string]any{"Name": "Test", "Show": false})).To(Succeed())
			Expect(s.ChangedFiles()).To(ConsistOf("maybe.txt", "present.txt"))
		})

		It("Should include files created by the write function", func() {
			s, err := New(Config{
				TargetDirectory: targetDir,
				SourceDirectory: absTestdata("with_write"),
			}, template.FuncMap{})
			Expect(err).ToNot(HaveOccurred())

			Expect(s.Render(nil)).To(Succeed())
			Expect(s.ChangedFiles()).To(ConsistOf("main.txt", "extra.txt"))
		})

		It("Should reset between renders", func() {
			s, err := New(Config{
				TargetDirectory: targetDir,
				Source: map[string]any{
					"first.txt": "one",
				},
			}, template.FuncMap{})
			Expect(err).ToNot(HaveOccurred())

			Expect(s.Render(nil)).To(Succeed())
			Expect(s.ChangedFiles()).To(ConsistOf("first.txt"))

			// second render into a new target
			secondTarget := filepath.Join(GinkgoT().TempDir(), "target2")
			s.cfg.TargetDirectory = secondTarget
			s.cfg.Source = map[string]any{
				"second.txt": "two",
			}

			Expect(s.Render(nil)).To(Succeed())
			Expect(s.ChangedFiles()).To(ConsistOf("second.txt"))
		})

		Context("With Jet engine", func() {
			It("Should track rendered files", func() {
				s, err := NewJet(Config{
					TargetDirectory: targetDir,
					SourceDirectory: absTestdata("simple"),
				}, map[string]jet.Func{})
				Expect(err).ToNot(HaveOccurred())

				Expect(s.Render(map[string]any{"Name": "World"})).To(Succeed())
				Expect(s.ChangedFiles()).To(ConsistOf("hello.txt"))
			})

			It("Should use forward slashes for nested paths", func() {
				s, err := NewJet(Config{
					TargetDirectory: targetDir,
					SourceDirectory: absTestdata("nested"),
				}, map[string]jet.Func{})
				Expect(err).ToNot(HaveOccurred())

				Expect(s.Render(map[string]any{"Name": "Top", "Value": "Deep"})).To(Succeed())
				Expect(s.ChangedFiles()).To(ConsistOf("top.txt", "sub/deep.txt"))
			})

			It("Should exclude skipped empty files", func() {
				s, err := NewJet(Config{
					TargetDirectory: targetDir,
					SourceDirectory: absTestdata("with_empty"),
					SkipEmpty:       true,
				}, map[string]jet.Func{})
				Expect(err).ToNot(HaveOccurred())

				Expect(s.Render(map[string]any{"Name": "Test", "Show": false})).To(Succeed())
				Expect(s.ChangedFiles()).To(ConsistOf("present.txt"))
			})

			It("Should include files created by the write function", func() {
				s, err := NewJet(Config{
					TargetDirectory: targetDir,
					SourceDirectory: absTestdata("jet_with_write"),
				}, map[string]jet.Func{})
				Expect(err).ToNot(HaveOccurred())

				Expect(s.Render(nil)).To(Succeed())
				Expect(s.ChangedFiles()).To(ConsistOf("main.txt", "extra.txt"))
			})
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

		It("Should reject paths that share a directory name prefix", func() {
			// Regression: target=/tmp/foo must not allow writes to /tmp/foobar/
			td := filepath.Join(GinkgoT().TempDir(), "foo")
			sibling := filepath.Join(GinkgoT().TempDir(), "foobar")
			Expect(os.MkdirAll(td, 0700)).To(Succeed())
			Expect(os.MkdirAll(sibling, 0700)).To(Succeed())

			s, err := New(Config{
				TargetDirectory:      td,
				MergeTargetDirectory: true,
				Source:               map[string]any{"f": "c"},
			}, nil)
			Expect(err).ToNot(HaveOccurred())

			err = s.saveFile(filepath.Join(sibling, "evil.txt"), "bad")
			Expect(err).To(MatchError(ContainSubstring("is not in target directory")))
		})
	})

	Describe("containedInDir", func() {
		It("Should match the directory itself", func() {
			Expect(containedInDir("/tmp/foo", "/tmp/foo")).To(BeTrue())
		})

		It("Should match children", func() {
			Expect(containedInDir("/tmp/foo/bar.txt", "/tmp/foo")).To(BeTrue())
		})

		It("Should reject sibling directories with shared prefix", func() {
			Expect(containedInDir("/tmp/foobar/evil.txt", "/tmp/foo")).To(BeFalse())
		})

		It("Should reject parent paths", func() {
			Expect(containedInDir("/tmp/evil.txt", "/tmp/foo")).To(BeFalse())
		})
	})

	Describe("validateSourcePath", func() {
		It("Should allow paths within the source directory", func() {
			s := &Scaffold{workingSource: "/tmp/source"}
			path, err := s.validateSourcePath("_partials/partial.txt")
			Expect(err).ToNot(HaveOccurred())
			Expect(path).To(Equal(filepath.Join("/tmp/source", "_partials/partial.txt")))
		})

		It("Should reject paths that escape the source directory", func() {
			s := &Scaffold{workingSource: "/tmp/source"}
			_, err := s.validateSourcePath("../../../etc/passwd")
			Expect(err).To(MatchError(ContainSubstring("is not in source directory")))
		})

		It("Should reject paths that use prefix tricks", func() {
			s := &Scaffold{workingSource: "/tmp/source"}
			_, err := s.validateSourcePath("../../sourcebar/evil.txt")
			Expect(err).To(MatchError(ContainSubstring("is not in source directory")))
		})
	})

	Describe("write template function path traversal", func() {
		It("Should reject traversal via write in Go templates", func() {
			s, err := New(Config{
				TargetDirectory: targetDir,
				Source: map[string]any{
					"evil.txt": `{{ write "../escape.txt" "bad" }}`,
				},
			}, template.FuncMap{})
			Expect(err).ToNot(HaveOccurred())

			err = s.Render(nil)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("is not in target directory"))
		})

		It("Should reject traversal via write in Jet templates", func() {
			s, err := NewJet(Config{
				TargetDirectory: targetDir,
				Source: map[string]any{
					"evil.txt": `{{ write("../escape.txt", "bad") }}`,
				},
			}, map[string]jet.Func{})
			Expect(err).ToNot(HaveOccurred())

			err = s.Render(nil)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("is not in target directory"))
		})
	})

	Describe("render template function path traversal", func() {
		It("Should reject traversal via render in Go templates", func() {
			s, err := New(Config{
				TargetDirectory: targetDir,
				Source: map[string]any{
					"evil.txt": `{{ render "../../../etc/passwd" . }}`,
				},
			}, template.FuncMap{})
			Expect(err).ToNot(HaveOccurred())

			err = s.Render(nil)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("is not in source directory"))
		})

		It("Should reject traversal via render in Jet templates", func() {
			s, err := NewJet(Config{
				TargetDirectory: targetDir,
				Source: map[string]any{
					"evil.txt": `{{ render("../../../etc/passwd", "x") }}`,
				},
			}, map[string]jet.Func{})
			Expect(err).ToNot(HaveOccurred())

			err = s.Render(nil)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("is not in source directory"))
		})
	})
})

type testLogger struct{}

func (t *testLogger) Debugf(format string, v ...any) {}
func (t *testLogger) Infof(format string, v ...any)  {}
