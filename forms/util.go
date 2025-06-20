// Copyright (c) 2023-2024, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package forms

import (
	"bytes"
	"os"
	"strings"
	"text/template"

	"github.com/choria-io/scaffold/internal/sprig"
	"github.com/jedib0t/go-pretty/v6/text"

	"github.com/AlecAivazis/survey/v2"
	terminal "golang.org/x/term"
)

func propertyEmptyVal(p Property) any {
	switch p.IfEmpty {
	case ArrayIfEmpty:
		return map[string]any{p.Name: []any{}}
	case ObjectIfEmpty:
		return map[string]any{p.Name: map[string]any{}}
	default:
		return map[string]any{}
	}
}
func askConfirmation(prompt string, dflt bool) (bool, error) {
	ans := dflt

	err := survey.AskOne(&survey.Confirm{
		Message: prompt,
		Default: dflt,
	}, &ans)

	return ans, err
}

func isTerminal() bool {
	return terminal.IsTerminal(int(os.Stdin.Fd())) && terminal.IsTerminal(int(os.Stdout.Fd()))
}

func isOneOf(val string, valid ...string) bool {
	for _, v := range valid {
		if val == v {
			return true
		}
	}
	return false
}

func renderTemplate(tmpl string, env map[string]any) (string, error) {
	t, err := template.New("form").Funcs(sprig.TxtFuncMap()).Parse(tmpl)
	if err != nil {
		return "", err
	}

	out := bytes.NewBuffer([]byte{})

	err = t.Execute(out, env)
	if err != nil {
		return "", err
	}

	return colorMarkup(out.String()), nil
}

// colorMarkup parses a string with color markup tags and returns a colorized string.
// Supports tags like {red}text{/red}, {blue}text{/blue}, etc.
// Supports all colors available in the go-pretty/text package.
// Supports nested and multiple color tags in a single string.
func colorMarkup(input string) string {
	colorMap := map[string]text.Color{
		"bold":      text.Bold,
		"black":     text.FgBlack,
		"red":       text.FgRed,
		"green":     text.FgGreen,
		"yellow":    text.FgYellow,
		"blue":      text.FgBlue,
		"magenta":   text.FgMagenta,
		"cyan":      text.FgCyan,
		"white":     text.FgWhite,
		"hiblack":   text.FgHiBlack,
		"hired":     text.FgHiRed,
		"higreen":   text.FgHiGreen,
		"hiyellow":  text.FgHiYellow,
		"hiblue":    text.FgHiBlue,
		"himagenta": text.FgHiMagenta,
		"hicyan":    text.FgHiCyan,
		"hiwhite":   text.FgHiWhite,
	}

	// Process innermost tags first to handle nesting properly
	result := input
	for {
		changed := false

		// Find innermost color tag (one that doesn't contain other opening tags)
		for i := 0; i < len(result); i++ {
			if result[i] != '{' {
				continue
			}

			// Find the end of this opening tag
			closePos := strings.Index(result[i:], "}")
			if closePos == -1 {
				continue
			}
			closePos += i

			colorName := result[i+1 : closePos]

			// Skip if this contains a slash (it's a closing tag)
			if strings.Contains(colorName, "/") {
				continue
			}

			// Find the corresponding closing tag
			closeTag := "{/" + colorName + "}"
			closeStart := strings.Index(result[closePos+1:], closeTag)
			if closeStart == -1 {
				continue
			}
			closeStart += closePos + 1

			// Check if this is innermost (no opening tags in between)
			content := result[closePos+1 : closeStart]
			if strings.Contains(content, "{") && !strings.HasPrefix(strings.TrimSpace(content[strings.Index(content, "{"):]), "/") {
				// This contains other opening tags, skip for now
				continue
			}

			// This is an innermost tag, process it
			fullMatch := result[i : closeStart+len(closeTag)]

			colorNameLower := strings.ToLower(colorName)
			if color, exists := colorMap[colorNameLower]; exists {
				coloredText := text.Colors{color}.Sprint(content)
				result = strings.Replace(result, fullMatch, coloredText, 1)
				changed = true
				break
			} else {
				// If color doesn't exist, just remove the tags
				result = strings.Replace(result, fullMatch, content, 1)
				changed = true
				break
			}
		}

		if !changed {
			break
		}
	}

	return result
}
