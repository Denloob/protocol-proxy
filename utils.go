package main

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
