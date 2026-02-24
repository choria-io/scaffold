// Copyright (c) 2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/choria-io/fisk"
	"github.com/choria-io/scaffold"
	"github.com/choria-io/scaffold/forms"
)

var (
	source         string
	target         string
	stringData     map[string]string
	jsonData       string
	formData       string
	engineString   string
	leftDelimiter  string
	rightDelimiter string
	skipEmpty      bool
	merge          bool
	post           map[string]string
	version        string
)

func main() {
	stringData = map[string]string{}
	post = map[string]string{}

	app := fisk.New("scaffold", "Renders Forms and Scaffolds")
	app.Version(version)

	app.Help = `
Create directories full of files based on a template called a scaffold.

Optionally supports loading data from a form via interactive prompts.
`
	render := app.Command("render", "Renders a scaffold using custom data").Action(renderAction)
	render.HelpLong(`
Scaffolds are directories full of Go or Jet templates that will be rendered as a whole.

Data will be passed to the templates, data can be obtained from the CLI, Environment, Forms or JSON files. The shell environment is available as ENVIRONMENT in the data passed to the scaffold.
`)
	render.Arg("scaffold", "The directory holding the scaffold to render").ExistingDirVar(&source)
	render.Arg("target", "The directory to write the result into").StringVar(&target)
	render.Arg("data", "Data to pass to the templates").StringMapVar(&stringData)
	render.Flag("engine", "The template engine to use (jet, go)").Default("go").EnumVar(&engineString, "jet", "go")
	render.Flag("json", "Loads data from a JSON file").PlaceHolder("FILE").ExistingFileVar(&jsonData)
	render.Flag("form", "Loads data from a form file").PlaceHolder("FILE").ExistingFileVar(&formData)
	render.Flag("left", "Left delimiter").Default("{{").StringVar(&leftDelimiter)
	render.Flag("right", "Right delimiter").Default("}}").StringVar(&rightDelimiter)
	render.Flag("skip-empty", "Skip empty files").Default("true").BoolVar(&skipEmpty)
	render.Flag("merge", "Merge result into an existing directory").Default("true").BoolVar(&merge)
	render.Flag("post", "Post processing steps").PlaceHolder("PATTERN=TOOL").StringMapVar(&post)

	app.MustParseWithUsage(os.Args[1:])
}

func renderAction(_ *fisk.ParseContext) error {
	data := map[string]any{}
	for k, v := range stringData {
		data[k] = v
	}

	envData := map[string]string{}
	for _, val := range os.Environ() {
		parts := strings.SplitN(val, "=", 2)
		if len(parts) != 2 {
			continue
		}
		envData[parts[0]] = parts[1]
	}
	data["ENVIRONMENT"] = envData

	if jsonData != "" {
		df, err := os.ReadFile(jsonData)
		if err != nil {
			return err
		}
		err = json.Unmarshal(df, &data)
		if err != nil {
			return err
		}
	}

	if formData != "" {
		form, err := forms.ProcessFile(formData, data)
		if err != nil {
			return err
		}
		for k, v := range form {
			data[k] = v
		}
	}

	var s *scaffold.Scaffold
	var err error

	cfg := scaffold.Config{
		TargetDirectory:      target,
		SourceDirectory:      source,
		CustomLeftDelimiter:  leftDelimiter,
		CustomRightDelimiter: rightDelimiter,
		SkipEmpty:            skipEmpty,
		MergeTargetDirectory: merge,
	}

	for k, v := range post {
		cfg.Post = append(cfg.Post, map[string]string{k: v})
	}

	if engineString == "jet" {
		s, err = scaffold.NewJet(cfg, nil)
	} else {
		s, err = scaffold.New(cfg, nil)
	}
	if err != nil {
		return err
	}

	changes, err := s.Render(data)
	if err != nil {
		return err
	}

	for _, f := range changes {
		fmt.Printf("%s: %s\n", f.Action, filepath.Join(target, f.Path))
	}

	return nil
}
