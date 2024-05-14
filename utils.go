package main

import "strings"

// Must returns the value of T if there's no error and panics otherwise
func Must[T any](t T, err error) T {
	if err != nil {
		panic(err)
	}

	return t
}

// IsCharacter checks if char is a displayable ascii text character
func IsCharacter(char byte) bool {
	return ' ' <= char && char <= '~'
}

func ExtractStrings(buffer []byte, minStringLength int) []string {
	var foundStrings []string

	var stringBegin int
	insideString := false

	for i, char := range buffer {
		if !IsCharacter(char) {
			if insideString && i-stringBegin >= minStringLength {
				foundStrings = append(foundStrings, string(buffer[stringBegin:i]))
				insideString = false
			}
			continue
		}

		if !insideString {
			insideString = true
			stringBegin = i
		}
	}

	if insideString && len(buffer)-stringBegin >= minStringLength {
		foundStrings = append(foundStrings, string(buffer[stringBegin:]))
	}

	return foundStrings
}

func CountLines(s string) int {
	if len(s) == 0 {
		return 0
	}

	count := strings.Count(s, "\n") + 1

	return count
}

// PutOnTheBottomOfView combines view with target, putting target on the bottom
// of the given height. If view + target has more lines than the given height
// or if height <= 0, only view is returned.
func PutOnTheBottomOfView(view, target string, height int) string {
	if height <= 0 {
		return view
	}

	lineCountOfView := CountLines(view)
	lineCountOfTarget := CountLines(target)

	if lineCountOfView+lineCountOfTarget > height {
		return view
	}

	pad := strings.Repeat("\n", height-lineCountOfView-lineCountOfTarget)
	if lineCountOfView != 0 {
		pad += "\n"
	}

	return view + pad + target
}
