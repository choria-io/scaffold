// Copyright (c) 2023-2024, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

// Package forms implements interactive terminal forms that collect user input
// and produce structured data. Forms are defined as YAML documents containing
// typed properties (string, bool, integer, float, password, object, array) that
// are presented to the user interactively. Properties support conditionals,
// validation expressions, enums, defaults, and nested sub-properties.
//
// The collected answers are assembled into a map[string]any result using an
// internal entry tree (see graph.go) that supports querying partially-built
// results, enabling conditional properties that reference earlier answers.
package forms

//go:generate mockgen -source forms.go -destination mock_test.go -package forms -typed

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strconv"
	"text/template"

	"github.com/AlecAivazis/survey/v2"
	"github.com/Masterminds/sprig/v3"
	"github.com/choria-io/scaffold/internal/validator"
	"gopkg.in/yaml.v3"
)

// surveyor abstracts the survey library for testability.
type surveyor interface {
	AskOne(p survey.Prompt, response interface{}, opts ...survey.AskOpt) error
}

type defaultSurveyor struct{}

func (d *defaultSurveyor) AskOne(p survey.Prompt, response interface{}, opts ...survey.AskOpt) error {
	return survey.AskOne(p, response, opts...)
}

type processOption func(*processor)

func withSurveyor(s surveyor) processOption {
	return func(p *processor) {
		p.surveyor = s
	}
}

func withIsTerminal(f func() bool) processOption {
	return func(p *processor) {
		p.isTerminal = f
	}
}

func withOutput(w io.Writer) processOption {
	return func(p *processor) {
		p.output = w
	}
}

// IfEmpty constants control what value is emitted when a property answer is empty.
const (
	ArrayIfEmpty  = "array"  // emit an empty array
	ObjectIfEmpty = "object" // emit an empty object
	AbsentIfEmpty = "absent" // omit the key entirely
)

// Type constants identify property types in form definitions.
const (
	StringType   = "string"
	BoolType     = "bool"
	IntType      = "integer"
	FloatType    = "float"
	PasswordType = "password"
	ObjectType   = "object"
	ArrayType    = "array"
)

// Form defines an interactive form with a name, description, and a list of properties
// to present to the user. The Description supports Go template syntax with Sprig functions
// and color markup tags like {red}text{/red}.
type Form struct {
	Name        string     `json:"name" yaml:"name"`
	Description string     `json:"description" yaml:"description"`
	Properties  []Property `json:"properties" yaml:"properties"`
}

// Property defines a single form field. Type determines the input method (string, bool,
// integer, float, password, object, array). Properties with sub-Properties create nested
// structures. ConditionalExpression is a validation expression evaluated against the current
// environment and collected input to decide whether to present this property.
type Property struct {
	Name                  string     `json:"name" yaml:"name"`
	Description           string     `json:"description" yaml:"description"`
	Help                  string     `json:"help" yaml:"help"`
	IfEmpty               string     `json:"empty" yaml:"empty"`
	Type                  string     `json:"type" yaml:"type"`
	ConditionalExpression string     `json:"conditional" yaml:"conditional"`
	ValidationExpression  string     `json:"validation" yaml:"validation"`
	Required              bool       `json:"required" yaml:"required"`
	Default               string     `json:"default" yaml:"default"`
	Enum                  []string   `json:"enum" yaml:"enum"`
	Properties            []Property `json:"properties" yaml:"properties"`
}

// RenderedDescription executes the property's Description as a Go template with Sprig
// functions against env, then applies color markup to the result.
func (p *Property) RenderedDescription(env map[string]any) (string, error) {
	t, err := template.New("property").Funcs(sprig.FuncMap()).Parse(p.Description)
	if err != nil {
		return "", err
	}

	buffer := bytes.NewBuffer([]byte{})
	err = t.Execute(buffer, env)
	if err != nil {
		return "", err
	}

	return colorMarkup(buffer.String()), nil
}

// processor holds the configuration needed to interactively process a form.
// The entry tree root is not stored here; it is passed explicitly through the
// ask methods so that the processor remains focused on user interaction.
type processor struct {
	env        map[string]any
	surveyor   surveyor
	isTerminal func() bool
	output     io.Writer
}

// ProcessReader reads YAML form data from r and processes it interactively.
func ProcessReader(r io.Reader, env map[string]any, opts ...processOption) (map[string]any, error) {
	fb, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	return ProcessBytes(fb, env, opts...)
}

// ProcessFile reads YAML form data from the file at path f and processes it interactively.
func ProcessFile(f string, env map[string]any, opts ...processOption) (map[string]any, error) {
	fb, err := os.ReadFile(f)
	if err != nil {
		return nil, err
	}

	return ProcessBytes(fb, env, opts...)
}

// ProcessBytes unmarshals f as a YAML form definition and processes it interactively.
func ProcessBytes(f []byte, env map[string]any, opts ...processOption) (map[string]any, error) {
	var form Form
	err := yaml.Unmarshal(f, &form)
	if err != nil {
		return nil, err
	}

	return ProcessForm(form, env, opts...)
}

// ProcessForm presents the form interactively on a terminal and returns the collected
// answers as a map. It requires a valid terminal (stdin and stdout). The env map provides
// template variables for property descriptions and conditional expressions.
func ProcessForm(f Form, env map[string]any, opts ...processOption) (map[string]any, error) {
	proc := &processor{
		env:        env,
		surveyor:   &defaultSurveyor{},
		isTerminal: isTerminal,
		output:     os.Stdout,
	}

	for _, o := range opts {
		o(proc)
	}

	if !proc.isTerminal() {
		return nil, fmt.Errorf("can only process forms on a valid terminal")
	}

	if len(f.Properties) == 0 {
		return nil, fmt.Errorf("no properties defined")
	}

	d, err := renderTemplate(f.Description, env)
	if err != nil {
		return nil, err
	}
	fmt.Fprintln(proc.output, d)

	fmt.Fprintln(proc.output)

	proc.surveyor.AskOne(&survey.Input{Message: "Press enter to start"}, &struct{}{})

	root := newObjectEntry(map[string]any{})

	err = proc.askProperties(f.Properties, root, root)
	if err != nil {
		return nil, err
	}

	_, res := root.combinedValue()
	result, ok := res.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("unexpected form result type %T", res)
	}
	return result, nil
}

// askArrayType collects array values for prop and attaches them to parent.
// Arrays with sub-properties produce []map[string]any; simple arrays produce []string.
func (p *processor) askArrayType(prop Property, parent entry, root entry) error {
	val, err := p.askArrayTypeProperty(prop, root)
	if err != nil {
		return err
	}

	np, err := parent.addChild(newObjectEntry(map[string]any{prop.Name: []any{}}))
	if err != nil {
		return err
	}

	switch nv := val.(type) {
	case []string:
		var n []any
		for _, v := range nv {
			n = append(n, v)
		}

		_, err = np.addChild(newArrayEntry(n))
		return err

	case nil:
		return nil

	default:
		nvm, ok := nv.([]map[string]any)
		if !ok {
			return fmt.Errorf("unexpected array property type %T", nv)
		}
		n := []any{}
		for _, v := range nvm {
			n = append(n, v)
		}

		_, err = np.addChild(newArrayEntry(n))
		return err
	}
}

// askObjWithProperties handles object or namespaced properties that have sub-properties.
// For ObjectType properties it loops, asking for a unique entry name each iteration and
// collecting sub-properties under that name. For untyped properties with sub-properties
// it collects one set of answers under prop.Name and returns.
func (p *processor) askObjWithProperties(prop Property, parent entry, root entry) error {
	d, err := prop.RenderedDescription(p.env)
	if err != nil {
		return err
	}
	fmt.Fprintln(p.output)
	fmt.Fprintln(p.output, d)
	fmt.Fprintln(p.output)

	firstEntry := true

	for {
		if prop.Type == ObjectType {
			// For required objects, skip confirmation on the first entry (at least one is mandatory).
			// For optional objects or after the first entry, always ask.
			if !firstEntry || !prop.Required {
				ok, err := p.askConfirmation(fmt.Sprintf("Add %s entry", prop.Name), false)
				if err != nil {
					return err
				}

				if !ok {
					_, err = parent.addChild(newObjectEntry(propertyEmptyVal(prop)))
					if err != nil {
						return err
					}
					return nil
				}
			}
		}

		var ans string

		if prop.Type == ObjectType {
			err := p.surveyor.AskOne(&survey.Input{
				Message: "Unique name for this entry",
				Help:    prop.Help,
			}, &ans, survey.WithValidator(survey.Required))
			if err != nil {
				return err
			}
		} else {
			ans = prop.Name
		}

		val, err := parent.addChild(newObjectEntry(map[string]any{ans: nil}))
		if err != nil {
			return err
		}

		err = p.askProperties(prop.Properties, val, root)
		if err != nil {
			return err
		}

		firstEntry = false

		// when type is empty we are not asking for a nested object, just one so we bail
		if prop.Type == "" {
			return nil
		}
	}
}

// askInt prompts for an integer value and adds it to parent.
func (p *processor) askInt(prop Property, parent entry) error {
	ans, err := p.askIntValue(prop)
	if err != nil {
		return err
	}

	_, err = parent.addChild(newObjectEntry(map[string]any{prop.Name: ans}))

	return err
}

// askFloat prompts for a float value and adds it to parent.
func (p *processor) askFloat(prop Property, parent entry) error {
	ans, err := p.askFloatValue(prop)
	if err != nil {
		return err
	}

	_, err = parent.addChild(newObjectEntry(map[string]any{prop.Name: ans}))

	return err
}

// askBool prompts for a boolean value and adds it to parent.
func (p *processor) askBool(prop Property, parent entry) error {
	ans, err := p.askBoolValue(prop)
	if err != nil {
		return err
	}

	_, err = parent.addChild(newObjectEntry(map[string]any{prop.Name: ans}))

	return err
}

// askString prompts for a string or password value and adds it to parent.
// Handles IfEmpty behavior: AbsentIfEmpty omits the key, other IfEmpty values
// emit a typed empty value, and non-empty answers are stored normally.
func (p *processor) askString(prop Property, parent entry) error {
	ans, err := p.askStringValue(prop)
	if err != nil {
		return err
	}

	switch {
	case ans == "" && prop.IfEmpty == AbsentIfEmpty:
	case ans == "" && prop.IfEmpty != "":
		_, err = parent.addChild(newObjectEntry(propertyEmptyVal(prop)))
	default:
		_, err = parent.addChild(newObjectEntry(map[string]any{prop.Name: ans}))
	}

	return err
}

// askProperties iterates over props, evaluates each property's conditional expression
// against root, and delegates to askProperty for those that should be presented.
func (p *processor) askProperties(props []Property, parent entry, root entry) error {
	for _, prop := range props {
		should, err := p.shouldProcess(prop, root)
		if err != nil {
			return err
		}
		if !should {
			continue
		}

		if err := p.askProperty(prop, parent, root); err != nil {
			return err
		}
	}

	return nil
}

// askProperty dispatches a single property to the appropriate type-specific handler.
func (p *processor) askProperty(prop Property, parent entry, root entry) error {
	switch {
	case prop.Type == ArrayType:
		return p.askArrayType(prop, parent, root)

	case isOneOf(prop.Type, ObjectType, "") && len(prop.Properties) > 0:
		return p.askObjWithProperties(prop, parent, root)

	case prop.Type == BoolType:
		return p.askBool(prop, parent)

	case prop.Type == IntType:
		return p.askInt(prop, parent)

	case prop.Type == FloatType:
		return p.askFloat(prop, parent)

	case isOneOf(prop.Type, StringType, PasswordType, ""):
		return p.askString(prop, parent)

	default:
		return fmt.Errorf("unsupported property type %q", prop.Type)
	}
}

// askStringEnum presents a select prompt with the property's Enum choices.
func (p *processor) askStringEnum(prop Property) (string, error) {
	var ans string
	var opts []survey.AskOpt

	if prop.Required {
		opts = append(opts, survey.WithValidator(survey.Required))
	}

	deflt := prop.Default
	if prop.Default == "" {
		deflt = prop.Enum[0]
	}

	err := p.surveyor.AskOne(&survey.Select{
		Message: prop.Name,
		Help:    prop.Help,
		Default: deflt,
		Options: prop.Enum,
	}, &ans, opts...)
	if err != nil {
		return "", err
	}

	return ans, nil
}

// askStringValue displays the property description, then prompts for a string
// (or password) value. Delegates to askStringEnum when the property has Enum values.
func (p *processor) askStringValue(prop Property) (string, error) {
	d, err := prop.RenderedDescription(p.env)
	if err != nil {
		return "", err
	}
	fmt.Fprintln(p.output)
	fmt.Fprintln(p.output, d)
	fmt.Fprintln(p.output)

	if len(prop.Enum) > 0 {
		return p.askStringEnum(prop)
	}

	var ans string
	var opts []survey.AskOpt

	if prop.Required {
		opts = append(opts, survey.WithValidator(survey.MinLength(1)))
	}

	if prop.ValidationExpression != "" {
		opts = append(opts, survey.WithValidator(validator.SurveyValidator(prop.ValidationExpression, prop.Required)))
	}

	if prop.Type == PasswordType {
		err = p.surveyor.AskOne(&survey.Password{
			Message: prop.Name,
			Help:    prop.Help,
		}, &ans, opts...)
	} else {
		err = p.surveyor.AskOne(&survey.Input{
			Message: prop.Name,
			Help:    prop.Help,
			Default: prop.Default,
		}, &ans, opts...)
	}
	if err != nil {
		return "", err
	}

	return ans, nil
}

// askFloatValue displays the property description and prompts for a float value,
// validating the input with an isFloat expression combined with any custom validation.
func (p *processor) askFloatValue(prop Property) (float64, error) {
	d, err := prop.RenderedDescription(p.env)
	if err != nil {
		return 0, err
	}
	fmt.Fprintln(p.output)
	fmt.Fprintln(p.output, d)
	fmt.Fprintln(p.output)

	var ans string

	validation := "isFloat(value)"
	if prop.ValidationExpression != "" {
		validation = fmt.Sprintf("%s && %s", validation, prop.ValidationExpression)
	}

	err = p.surveyor.AskOne(&survey.Input{
		Message: prop.Name,
		Help:    prop.Help,
		Default: prop.Default,
	}, &ans, survey.WithValidator(validator.SurveyValidator(validation, true)))
	if err != nil {
		return 0, err
	}

	return strconv.ParseFloat(ans, 64)
}

// askIntValue displays the property description and prompts for an integer value,
// validating the input with an isInt expression combined with any custom validation.
func (p *processor) askIntValue(prop Property) (int, error) {
	d, err := prop.RenderedDescription(p.env)
	if err != nil {
		return 0, err
	}
	fmt.Fprintln(p.output)
	fmt.Fprintln(p.output, d)
	fmt.Fprintln(p.output)

	var ans string

	validation := "isInt(value)"
	if prop.ValidationExpression != "" {
		validation = fmt.Sprintf("%s && %s", validation, prop.ValidationExpression)
	}

	err = p.surveyor.AskOne(&survey.Input{
		Message: prop.Name,
		Help:    prop.Help,
		Default: prop.Default,
	}, &ans, survey.WithValidator(validator.SurveyValidator(validation, true)))
	if err != nil {
		return 0, err
	}

	return strconv.Atoi(ans)
}

// askBoolValue displays the property description and prompts for a boolean confirmation.
func (p *processor) askBoolValue(prop Property) (bool, error) {
	d, err := prop.RenderedDescription(p.env)
	if err != nil {
		return false, err
	}
	fmt.Fprintln(p.output)
	fmt.Fprintln(p.output, d)
	fmt.Fprintln(p.output)

	var ans bool
	var dflt bool

	if prop.Default != "" {
		dflt, err = strconv.ParseBool(prop.Default)
		if err != nil {
			return false, err
		}
	}

	err = p.surveyor.AskOne(&survey.Confirm{
		Message: prop.Name,
		Help:    prop.Help,
		Default: dflt,
	}, &ans)
	if err != nil {
		return false, err
	}

	return ans, nil
}

// askArrayTypeProperty collects array entries by repeatedly prompting the user.
// For properties with sub-properties it returns []map[string]any; for simple
// properties it returns []string. Returns nil when the user declines and
// IfEmpty is AbsentIfEmpty.
func (p *processor) askArrayTypeProperty(prop Property, root entry) (any, error) {
	switch {
	case len(prop.Properties) > 0:
		answer := []map[string]any{}

		for {
			if len(answer) > 0 || !prop.Required {
				prompt := fmt.Sprintf("Add additional '%s' entry", prop.Name)
				if len(answer) == 0 {
					prompt = fmt.Sprintf("Add first '%s' entry", prop.Name)
				}

				ok, err := p.askConfirmation(prompt, false)
				if err != nil {
					return nil, err
				}
				if !ok {
					if len(answer) > 0 {
						return answer, nil
					}

					if prop.IfEmpty == AbsentIfEmpty {
						return nil, nil
					}

					return []map[string]any{propertyEmptyVal(prop)}, nil
				}
			}

			val := newObjectEntry(map[string]any{})
			err := p.askProperties(prop.Properties, val, root)
			if err != nil {
				return nil, err
			}

			_, cv := val.combinedValue()
			m, ok := cv.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("unexpected combined value type %T", cv)
			}
			answer = append(answer, m)
		}

	default:
		var ans []string
		var err error

		for {
			var val string
			var ok = true

			if len(ans) > 0 || !prop.Required {
				prompt := fmt.Sprintf("Add additional '%s' entry", prop.Name)
				if len(ans) == 0 {
					prompt = fmt.Sprintf("Add first '%s' entry", prop.Name)
				}

				ok, err = p.askConfirmation(prompt, false)
				if err != nil {
					return nil, err
				}
			}

			if ok {
				val, err = p.askStringValue(prop)
				if err != nil {
					return nil, err
				}
			}

			if val == "" {
				break
			}

			ans = append(ans, val)
		}

		fmt.Fprintln(p.output)

		return ans, nil
	}
}

// shouldProcess evaluates the property's ConditionalExpression against the current
// environment merged with the answers collected so far (available as "input"/"Input").
// Returns true when there is no conditional or when the expression evaluates to true.
func (p *processor) shouldProcess(prop Property, root entry) (bool, error) {
	if prop.ConditionalExpression == "" {
		return true, nil
	}

	env := make(map[string]any)
	for k, v := range p.env {
		env[k] = v
	}

	_, env["input"] = root.combinedValue()
	env["Input"] = env["input"]

	return validator.Validate(env, prop.ConditionalExpression)
}
