// Copyright (c) 2023-2024, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package forms

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strconv"
	"text/template"

	"github.com/AlecAivazis/survey/v2"
	"github.com/choria-io/scaffold/internal/sprig"
	"github.com/choria-io/scaffold/internal/validator"
	"gopkg.in/yaml.v3"
)

const (
	ArrayIfEmpty  = "array"
	ObjectIfEmpty = "object"
	AbsentIfEmpty = "absent"
	StringType    = "string"
	BoolType      = "bool"
	IntType       = "integer"
	FloatType     = "float"
	PasswordType  = "password"
	ObjectType    = "object"
	ArrayType     = "array"
)

type Form struct {
	Name        string     `json:"name" yaml:"name"`
	Description string     `json:"description" yaml:"description"`
	Properties  []Property `json:"properties" yaml:"properties"`
}

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

type processor struct {
	form Form
	val  entry
	env  map[string]any
}

// ProcessReader reads all data from r and ProcessForm() it as YAML
func ProcessReader(r io.Reader, env map[string]any) (map[string]any, error) {
	fb, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	return ProcessBytes(fb, env)
}

// ProcessFile reads f and ProcessForm() it as YAML
func ProcessFile(f string, env map[string]any) (map[string]any, error) {
	fb, err := os.ReadFile(f)
	if err != nil {
		return nil, err
	}

	return ProcessBytes(fb, env)
}

// ProcessBytes treats f as a YAML document and ProcessForm() it
func ProcessBytes(f []byte, env map[string]any) (map[string]any, error) {
	var form Form
	err := yaml.Unmarshal(f, &form)
	if err != nil {
		panic(err)
	}

	return ProcessForm(form, env)
}

// ProcessForm processes the form and return a data structure with the answers
func ProcessForm(f Form, env map[string]any) (map[string]any, error) {
	if !isTerminal() {
		return nil, fmt.Errorf("can only process forms on a valid terminal")
	}

	if len(f.Properties) == 0 {
		return nil, fmt.Errorf("no properties defined")
	}

	proc := &processor{
		form: f,
		val:  newObjectEntry(map[string]any{}),
		env:  env,
	}

	d, err := renderTemplate(f.Description, env)
	if err != nil {
		return nil, err
	}
	fmt.Println(d)

	fmt.Println()

	survey.AskOne(&survey.Input{Message: "Press enter to start"}, &struct{}{})

	err = proc.askProperties(f.Properties, proc.val)
	if err != nil {
		return nil, err
	}

	_, res := proc.val.combinedValue()
	return res.(map[string]any), nil
}

func (p *processor) askArrayType(prop Property, parent entry) error {
	val, err := p.askArrayTypeProperty(prop)
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
		n := []any{}
		for _, v := range nv.([]map[string]any) {
			n = append(n, v)
		}

		_, err = np.addChild(newArrayEntry(n))
		return err
	}
}

func (p *processor) askObjWithProperties(prop Property, parent entry) error {
	d, err := prop.RenderedDescription(p.env)
	if err != nil {
		return err
	}
	fmt.Println()
	fmt.Println(d)
	fmt.Println()

	for {
		if !prop.Required && prop.Type == ObjectType {
			ok, err := askConfirmation(fmt.Sprintf("Add %s entry", prop.Name), false)
			if err != nil {
				return err
			}

			if !ok {
				_, err = parent.addChild(newObjectEntry(propertyEmptyVal(prop).(map[string]any)))
				if err != nil {
					return err
				}
				return nil
			}
		}

		var ans string

		if prop.Type == ObjectType {
			err := survey.AskOne(&survey.Input{
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

		err = p.askProperties(prop.Properties, val)
		if err != nil {
			return err
		}

		// when type is empty we are not asking for a nested object, just one so we bail
		if prop.Type == "" {
			return nil
		}
	}
}

func (p *processor) askInt(prop Property, parent entry) error {
	ans, err := p.askIntValue(prop)
	if err != nil {
		return err
	}

	_, err = parent.addChild(newObjectEntry(map[string]any{prop.Name: ans}))

	return err
}

func (p *processor) askFloat(prop Property, parent entry) error {
	ans, err := p.askFloatValue(prop)
	if err != nil {
		return err
	}

	_, err = parent.addChild(newObjectEntry(map[string]any{prop.Name: ans}))

	return err
}

func (p *processor) askBool(prop Property, parent entry) error {
	ans, err := p.askBoolValue(prop)
	if err != nil {
		return err
	}

	_, err = parent.addChild(newObjectEntry(map[string]any{prop.Name: ans}))

	return err
}

func (p *processor) askString(prop Property, parent entry) error {
	ans, err := p.askStringValue(prop)
	if err != nil {
		return err
	}

	switch {
	case ans == "" && prop.IfEmpty == AbsentIfEmpty:
	case ans == "" && prop.IfEmpty != "":
		_, err = parent.addChild(newObjectEntry(propertyEmptyVal(prop).(map[string]any)))
	default:
		_, err = parent.addChild(newObjectEntry(map[string]any{prop.Name: ans}))
	}

	return err
}

func (p *processor) askProperties(props []Property, parent entry) error {
	for _, prop := range props {
		should, err := p.shouldProcess(prop)
		if err != nil {
			return err
		}
		if !should {
			continue
		}

		switch {
		case prop.Type == ArrayType:
			err = p.askArrayType(prop, parent)

		case isOneOf(prop.Type, ObjectType, "") && len(prop.Properties) > 0:
			err = p.askObjWithProperties(prop, parent)

		case prop.Type == BoolType:
			err = p.askBool(prop, parent)

		case prop.Type == IntType:
			err = p.askInt(prop, parent)

		case prop.Type == FloatType:
			err = p.askFloat(prop, parent)

		case isOneOf(prop.Type, StringType, PasswordType, ""): // added to parent as a single item object entry
			err = p.askString(prop, parent)
		}

		if err != nil {
			return err
		}
	}

	return nil
}

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

	err := survey.AskOne(&survey.Select{
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

func (p *processor) askStringValue(prop Property) (string, error) {
	d, err := prop.RenderedDescription(p.env)
	if err != nil {
		return "", err
	}
	fmt.Println()
	fmt.Println(d)
	fmt.Println()

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
		err = survey.AskOne(&survey.Password{
			Message: prop.Name,
			Help:    prop.Help,
		}, &ans, opts...)
	} else {
		err = survey.AskOne(&survey.Input{
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

func (p *processor) askFloatValue(prop Property) (float64, error) {
	d, err := prop.RenderedDescription(p.env)
	if err != nil {
		return 0, err
	}
	fmt.Println()
	fmt.Println(d)
	fmt.Println()

	var ans string

	validation := "isFloat(value)"
	if prop.ValidationExpression != "" {
		validation = fmt.Sprintf("%s && %s", validation, prop.ValidationExpression)
	}

	err = survey.AskOne(&survey.Input{
		Message: prop.Name,
		Help:    prop.Help,
		Default: prop.Default,
	}, &ans, survey.WithValidator(validator.SurveyValidator(validation, true)))
	if err != nil {
		return 0, err
	}

	return strconv.ParseFloat(ans, 64)
}

func (p *processor) askIntValue(prop Property) (int, error) {
	d, err := prop.RenderedDescription(p.env)
	if err != nil {
		return 0, err
	}
	fmt.Println()
	fmt.Println(d)
	fmt.Println()

	var ans string

	validation := "isInt(value)"
	if prop.ValidationExpression != "" {
		validation = fmt.Sprintf("%s && %s", validation, prop.ValidationExpression)
	}

	err = survey.AskOne(&survey.Input{
		Message: prop.Name,
		Help:    prop.Help,
		Default: prop.Default,
	}, &ans, survey.WithValidator(validator.SurveyValidator(validation, true)))
	if err != nil {
		return 0, err
	}

	return strconv.Atoi(ans)
}

func (p *processor) askBoolValue(prop Property) (bool, error) {
	d, err := prop.RenderedDescription(p.env)
	if err != nil {
		return false, err
	}
	fmt.Println()
	fmt.Println(d)
	fmt.Println()

	var ans bool
	var dflt bool

	if prop.Default != "" {
		dflt, err = strconv.ParseBool(prop.Default)
		if err != nil {
			return false, err
		}
	}

	err = survey.AskOne(&survey.Confirm{
		Message: prop.Name,
		Help:    prop.Help,
		Default: dflt,
	}, &ans)
	if err != nil {
		return false, err
	}

	return ans, nil
}

func (p *processor) askArrayTypeProperty(prop Property) (any, error) {
	switch {
	case len(prop.Properties) > 0:
		answer := []map[string]any{}

		for {
			if len(answer) > 0 || !prop.Required {
				prompt := fmt.Sprintf("Add additional '%s' entry", prop.Name)
				if len(answer) == 0 {
					prompt = fmt.Sprintf("Add first '%s' entry", prop.Name)
				}

				ok, err := askConfirmation(prompt, false)
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

					return []map[string]any{propertyEmptyVal(prop).(map[string]any)}, nil
				}
			}

			val := newObjectEntry(map[string]any{})
			err := p.askProperties(prop.Properties, val)
			if err != nil {
				return nil, err
			}

			_, cv := val.combinedValue()
			answer = append(answer, cv.(map[string]any))
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

				ok, err = askConfirmation(prompt, false)
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

		fmt.Println()

		return ans, nil
	}
}

func (p *processor) shouldProcess(prop Property) (bool, error) {
	if prop.ConditionalExpression == "" {
		return true, nil
	}

	env := make(map[string]any)
	for k, v := range p.env {
		env[k] = v
	}

	_, env["input"] = p.val.combinedValue()
	env["Input"] = env["input"]

	return validator.Validate(env, prop.ConditionalExpression)
}
