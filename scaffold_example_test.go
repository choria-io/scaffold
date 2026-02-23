package scaffold_test

import (
	"fmt"
	"os"
	"path/filepath"
	"text/template"

	"github.com/choria-io/scaffold"
)

func Example() {
	base, _ := os.MkdirTemp("", "scaffold-example-")
	defer os.RemoveAll(base)
	target := filepath.Join(base, "output")

	s, err := scaffold.New(scaffold.Config{
		TargetDirectory: target,
		SkipEmpty:       true,
		Source: map[string]any{
			"README.md": "# {{ .Name }}\n\n{{ .Description }}\n",
			"lib": map[string]any{
				"main.go": `package {{ .Package }}
`,
			},
			"empty.txt": "{{ if .Include }}content{{ end }}",
		},
	}, template.FuncMap{})
	if err != nil {
		panic(err)
	}

	err = s.Render(map[string]any{
		"Name":        "My Project",
		"Description": "A scaffolded project.",
		"Package":     "main",
		"Include":     false,
	})
	if err != nil {
		panic(err)
	}

	readme, _ := os.ReadFile(filepath.Join(target, "README.md"))
	fmt.Println(string(readme))

	main, _ := os.ReadFile(filepath.Join(target, "lib", "main.go"))
	fmt.Println(string(main))

	_, statErr := os.Stat(filepath.Join(target, "empty.txt"))
	fmt.Println("empty.txt exists:", !os.IsNotExist(statErr))

	fmt.Println("changed:", s.ChangedFiles())

	// Output:
	// # My Project
	//
	// A scaffolded project.
	//
	// package main
	//
	// empty.txt exists: false
	// changed: [README.md lib/main.go]
}
