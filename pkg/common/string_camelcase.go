// The support for split camelcase word to a list of words implementated by
// github.com/fatih/camelcase.
// Original file: https://github.com/fatih/camelcase/blob/master/camelcase.go
// Give credit where credit is due.

package common

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// Split splits the camelcase word and returns a list of words. It also
// supports digits. Both lower camel case and upper camel case are supported.
// For more info please check: http://en.wikipedia.org/wiki/CamelCase
//
// Examples
//
//	"" =>                     [""]
//	"lowercase" =>            ["lowercase"]
//	"Class" =>                ["Class"]
//	"MyClass" =>              ["My", "Class"]
//	"MyC" =>                  ["My", "C"]
//	"HTML" =>                 ["HTML"]
//	"PDFLoader" =>            ["PDF", "Loader"]
//	"AString" =>              ["A", "String"]
//	"SimpleXMLParser" =>      ["Simple", "XML", "Parser"]
//	"vimRPCPlugin" =>         ["vim", "RPC", "Plugin"]
//	"GL11Version" =>          ["GL", "11", "Version"]
//	"99Bottles" =>            ["99", "Bottles"]
//	"May5" =>                 ["May", "5"]
//	"BFG9000" =>              ["BFG", "9000"]
//	"BöseÜberraschung" =>     ["Böse", "Überraschung"]
//	"Two  spaces" =>          ["Two", "  ", "spaces"]
//	"BadUTF8\xe2\xe2\xa1" =>  ["BadUTF8\xe2\xe2\xa1"]
//
// Splitting rules
//
//  1. If string is not valid UTF-8, return it without splitting as
//     single item array.
//  2. Assign all unicode characters into one of 4 sets: lower case
//     letters, upper case letters, numbers, and all other characters.
//  3. Iterate through characters of string, introducing splits
//     between adjacent characters that belong to different sets.
//  4. Iterate through array of split strings, and if a given string
//     is upper case:
//     if subsequent string is lower case:
//     move last character of upper case string to beginning of
//     lower case string
func Split(src string) (entries []string) {
	// don't split invalid utf8
	if !utf8.ValidString(src) {
		return []string{src}
	}
	entries = []string{}
	var runes [][]rune
	lastClass := 0
	class := 0
	// split into fields based on class of unicode character
	for _, r := range src {
		switch true {
		case unicode.IsLower(r):
			class = 1
		case unicode.IsUpper(r):
			class = 2
		case unicode.IsDigit(r):
			class = 3
		default:
			class = 4
		}
		if class == lastClass {
			runes[len(runes)-1] = append(runes[len(runes)-1], r)
		} else {
			runes = append(runes, []rune{r})
		}
		lastClass = class
	}
	// handle upper case -> lower case sequences, e.g.
	// "PDFL", "oader" -> "PDF", "Loader"
	for i := 0; i < len(runes)-1; i++ {
		if unicode.IsUpper(runes[i][0]) && unicode.IsLower(runes[i+1][0]) {
			runes[i+1] = append([]rune{runes[i][len(runes[i])-1]}, runes[i+1]...)
			runes[i] = runes[i][:len(runes[i])-1]
		}
	}
	// construct []string from results
	for _, s := range runes {
		if len(s) > 0 {
			entries = append(entries, string(s))
		}
	}
	return entries
}

func camelToConcatenatedStr(camel string, sep rune) string {
	runes := []rune(camel)
	length := len(runes)

	out := make([]rune, 0, length)
	for i := 0; i < length; i++ {
		if i > 0 && unicode.IsUpper(runes[i]) &&
			((i+1 < length && unicode.IsLower(runes[i+1])) || unicode.IsLower(runes[i-1])) {
			out = append(out, sep)
		}
		out = append(out, unicode.ToLower(runes[i]))
	}

	return string(out)
}

// CamelToKebab cast camel case to kebab case
func CamelToKebab(camel string) string {
	return camelToConcatenatedStr(camel, '-')
}

// CamelToSnake cast camel case to snake case
func CamelToSnake(camel string) string {
	return camelToConcatenatedStr(camel, '_')
}

// SnakeToCamel cast snake case to camel case
func SnakeToCamel(snake string) string {
	return strings.Join(
		strings.Split(
			strings.Title(
				strings.Join(
					strings.Split(snake, "_"),
					" "),
			), " ",
		),
		"")
}

// FormattedCamel cast to formatted cammel.
// For example: VirtualMachineID to VirtualMachineId
func FormattedCamel(str string) string {
	return SnakeToCamel(CamelToSnake(str))
}

// CamelListToSnakeList cast each string in camel list to snake case.
func CamelListToSnakeList(camelList []string) []string {
	snakeList := make([]string, len(camelList))
	for i := range camelList {
		snakeList[i] = CamelToSnake(camelList[i])
	}
	return snakeList
}

// SnakeListToCamelList cast each string in snake list to camel case.
func SnakeListToCamelList(snakeList []string) []string {
	camelList := make([]string, len(snakeList))
	for i := range snakeList {
		camelList[i] = SnakeToCamel(snakeList[i])
	}
	return camelList
}
