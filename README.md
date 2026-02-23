## Form and Scaffold Utilities for Golang

This project contains 2 main components:

 * `scaffold` generates files or entire directory hierarchies from complex data
 * `forms` is a guided input form system for the terminal that can generate arbitrarily nested complex data

Together they allow building Wizard-style file generators.

See [App Builder](https://github.com/choria-io/appbuilder) for these projects in use in an end-user product.

## Scaffold

The `scaffold` package renders directory trees from templates. Templates can
be supplied from a directory on disk or as in-memory maps. Both Go
`text/template` and [Jet](https://github.com/CloudyKit/jet) template engines
are supported.

### Basic usage

```go
s, err := scaffold.New(scaffold.Config{
    TargetDirectory: "/tmp/myproject",
    Source: map[string]any{
        "README.md": "# {{ .Name }}\n",
        "src": map[string]any{
            "main.go": "package {{ .Package }}\n",
        },
    },
}, template.FuncMap{})
if err != nil {
    return err
}

err = s.Render(map[string]any{
    "Name":    "My Project",
    "Package": "main",
})
```

This produces:

```
/tmp/myproject/
  README.md
  src/
    main.go
```

To use the Jet engine instead, call `scaffold.NewJet` with `map[string]jet.Func`
in place of `template.FuncMap`.

### Source directory

Instead of in-memory maps, templates can be read from a directory. The
directory structure is mirrored into the target:

```go
s, err := scaffold.New(scaffold.Config{
    TargetDirectory: "/tmp/myproject",
    SourceDirectory: "/path/to/templates",
}, template.FuncMap{})
```

### Configuration options

| Field                  | Description                                                                 |
|------------------------|-----------------------------------------------------------------------------|
| `target`               | Output directory (required)                                                 |
| `source_directory`     | Read templates from this directory (mutually exclusive with `source`)        |
| `source`               | In-memory template map (mutually exclusive with `source_directory`)          |
| `merge_target_directory` | Allow writing into an existing target directory                           |
| `skip_empty`           | Omit files that are empty after rendering                                   |
| `left_delimiter` / `right_delimiter` | Custom template delimiters (both must be set)                 |
| `post`                 | Post-processing commands matched by file glob                               |

### Built-in template functions

All [Sprig](https://masterminds.github.io/sprig/) functions are available when
using the Go template engine. Two additional functions are provided in both
engines:

**`write`** creates an extra file in the target directory from within a template:

```
{{/* Go template */}}
{{ write "extra.txt" "file content" }}
```

```
{{/* Jet template */}}
{{ write("extra.txt", "file content") }}
```

**`render`** evaluates another template file from the source directory and
returns its output as a string. The partial is rendered using the same engine
as the calling template:

```
{{/* Go template */}}
{{ render "_partials/header.txt" . }}
```

```
{{/* Jet template */}}
{{ render("_partials/header.txt", .) }}
```

### Partials

Any directory named `_partials` is excluded from the rendered output. Files
inside `_partials` are available to the `render` function but are not copied
to the target.

### Post-processing

The `post` configuration runs commands on rendered files that match a glob
pattern. Use `{}` as a placeholder for the file path; if omitted the path is
appended as the last argument:

```go
Post: []map[string]string{
    {"*.go": "gofmt -w {}"},
    {"*.sh": "chmod +x"},
},
```

### Merging into existing directories

Set `merge_target_directory` to render into a directory that already exists.
Only files whose content has changed are written; unchanged files are left
untouched.

### Changed files

After rendering, `ChangedFiles()` returns the list of files that were created
or modified, with paths relative to the target directory:

```go
err = s.Render(data)
for _, f := range s.ChangedFiles() {
    fmt.Println("changed:", f)
}
```

### Dry-run with RenderNoop

`RenderNoop` performs a full render into a temporary directory and compares the
result against the real target without modifying it. Each file is reported with
an action:

```go
plan, err := s.RenderNoop(data)
for _, f := range plan {
    fmt.Printf("%s %s\n", f.Action, f.Path) // add, update, equal, or remove
}
```

See the Go package documentation for full API details.

## Forms

The `forms` package provides interactive terminal forms that collect structured
user input. Forms are defined in YAML and produce `map[string]any` results
suitable for passing directly to the scaffold renderer.

### Defining a form

A form has a name, description, and a list of properties. Each property becomes
a prompt in the terminal.

```yaml
name: project
description: Create a new project
properties:
  - name: project_name
    description: The name of the project
    type: string
    required: true

  - name: license
    description: Choose a license
    type: string
    enum:
      - Apache-2.0
      - MIT
      - GPL-3.0
    default: Apache-2.0

  - name: port
    description: Default listen port
    type: integer
    default: "8080"
    validation: "value > 0 && value < 65536"

  - name: enable_tls
    description: Enable TLS support
    type: bool
    default: "true"

  - name: authors
    description: Project authors
    type: array

  - name: database
    description: Database connection settings
    properties:
      - name: host
        description: Database hostname
        type: string
        default: localhost
      - name: port
        description: Database port
        type: integer
        default: "5432"
```

### Property types

| Type       | Description                                    |
|------------|------------------------------------------------|
| `string`   | Free-text input, optionally limited by `enum`  |
| `password` | Masked string input                            |
| `integer`  | Whole number, validated automatically          |
| `float`    | Decimal number, validated automatically        |
| `bool`     | Yes/no confirmation                            |
| `array`    | Collects multiple values, including objects     |
| `object`   | Named group of nested properties               |

### Property options

| Field          | Description                                                              |
|----------------|--------------------------------------------------------------------------|
| `required`     | Prompt cannot be skipped                                                 |
| `default`      | Pre-filled value shown to the user                                       |
| `enum`         | Restrict input to a list of choices (string type)                        |
| `help`         | Extended help text shown on demand                                       |
| `validation`   | An [expr](https://github.com/expr-lang/expr) expression; `value` holds the input |
| `conditional`  | An expr expression controlling whether the property is shown; `input` holds answers so far |
| `empty`        | Behaviour when the answer is empty: `absent` omits the key, `array` or `object` sets an empty container |
| `properties`   | Nested properties for `object` and `array` types                         |

### Processing a form

```go
// From a YAML file
data, err := forms.ProcessFile("form.yaml", env)

// From bytes
data, err := forms.ProcessBytes(yamlBytes, env)

// From a parsed Form struct
data, err := forms.ProcessForm(form, env)
```

The `env` map is passed to template expressions in descriptions and is also
available in conditional expressions. The returned `map[string]any` contains
all collected answers keyed by property name.

### Descriptions with templates and color

Property descriptions support Go templates with Sprig functions and a simple
color markup syntax:

```yaml
description: >
  Configure the {green}{{ .ProjectName }}{/green} database connection.
```

Available colors: `black`, `red`, `green`, `yellow`, `blue`, `magenta`, `cyan`,
`white`, and their `hi` variants (e.g. `hired`). `bold` is also supported.

See the Go package documentation for full API details.