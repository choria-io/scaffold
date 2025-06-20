// Copyright (c) 2023-2024, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package forms

import (
	"github.com/jedib0t/go-pretty/v6/text"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("ColorMarkup", func() {
	Describe("colorMarkup function", func() {
		It("should handle no color markup", func() {
			input := "Hello World"
			expected := "Hello World"
			result := colorMarkup(input)
			Expect(result).To(Equal(expected))
		})

		It("should handle single color tag", func() {
			input := "{red}Hello{/red} World"
			expected := text.Colors{text.FgRed}.Sprint("Hello") + " World"
			result := colorMarkup(input)
			Expect(result).To(Equal(expected))
		})

		It("should handle multiple color tags", func() {
			input := "{red}Hello{/red} {blue}World{/blue}"
			expected := text.Colors{text.FgRed}.Sprint("Hello") + " " + text.Colors{text.FgBlue}.Sprint("World")
			result := colorMarkup(input)
			Expect(result).To(Equal(expected))
		})

		It("should handle nested color tags", func() {
			input := "{red}Outer {green}Inner{/green} Text{/red}"
			expected := text.Colors{text.FgRed}.Sprint("Outer " + text.Colors{text.FgGreen}.Sprint("Inner") + " Text")
			result := colorMarkup(input)
			Expect(result).To(Equal(expected))
		})

		It("should handle case insensitive colors", func() {
			input := "{RED}Hello{/RED} {Blue}World{/Blue}"
			expected := text.Colors{text.FgRed}.Sprint("Hello") + " " + text.Colors{text.FgBlue}.Sprint("World")
			result := colorMarkup(input)
			Expect(result).To(Equal(expected))
		})

		It("should handle high intensity colors", func() {
			input := "{hired}Error{/hired} {higreen}Success{/higreen}"
			expected := text.Colors{text.FgHiRed}.Sprint("Error") + " " + text.Colors{text.FgHiGreen}.Sprint("Success")
			result := colorMarkup(input)
			Expect(result).To(Equal(expected))
		})

		It("should remove invalid color tags", func() {
			input := "{invalid}Text{/invalid}"
			expected := "Text"
			result := colorMarkup(input)
			Expect(result).To(Equal(expected))
		})

		It("should handle mixed valid and invalid colors", func() {
			input := "{red}Valid{/red} {invalid}Invalid{/invalid} {blue}Another{/blue}"
			expected := text.Colors{text.FgRed}.Sprint("Valid") + " Invalid " + text.Colors{text.FgBlue}.Sprint("Another")
			result := colorMarkup(input)
			Expect(result).To(Equal(expected))
		})

		It("should handle empty color tag", func() {
			input := "{red}{/red}"
			expected := text.Colors{text.FgRed}.Sprint("")
			result := colorMarkup(input)
			Expect(result).To(Equal(expected))
		})

		It("should handle all standard colors", func() {
			input := "{black}black{/black} {red}red{/red} {green}green{/green} {yellow}yellow{/yellow} {blue}blue{/blue} {magenta}magenta{/magenta} {cyan}cyan{/cyan} {white}white{/white}"
			expected := text.Colors{text.FgBlack}.Sprint("black") + " " +
				text.Colors{text.FgRed}.Sprint("red") + " " +
				text.Colors{text.FgGreen}.Sprint("green") + " " +
				text.Colors{text.FgYellow}.Sprint("yellow") + " " +
				text.Colors{text.FgBlue}.Sprint("blue") + " " +
				text.Colors{text.FgMagenta}.Sprint("magenta") + " " +
				text.Colors{text.FgCyan}.Sprint("cyan") + " " +
				text.Colors{text.FgWhite}.Sprint("white")
			result := colorMarkup(input)
			Expect(result).To(Equal(expected))
		})

		It("should handle complex nesting and preserve all text content", func() {
			input := "{red}Start {blue}Middle {green}End{/green} More{/blue} Final{/red}"
			result := colorMarkup(input)

			// The function should process innermost tags first
			// This is a complex case that tests the iterative processing
			Expect(result).To(ContainSubstring("Start"))
			Expect(result).To(ContainSubstring("Middle"))
			Expect(result).To(ContainSubstring("End"))
			Expect(result).To(ContainSubstring("More"))
			Expect(result).To(ContainSubstring("Final"))
		})
	})
})
