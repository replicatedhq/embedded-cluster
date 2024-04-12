package util

// NameWithLengthLimit returns a string that is the concatenation of the prefix and suffix strings.
// If the total length of the concatenated string is less than or equal to 63 characters, the concatenated string is returned.
// If the total length of the concatenated string is greater than 63 characters, characters are removed from the middle of the string to reduce the total length to 63 characters.
func NameWithLengthLimit(prefix, suffix string) string {
	candidate := prefix + suffix
	if len(candidate) <= 63 {
		return candidate
	}

	// remove characters from the middle of the string to reduce the total length to 63 characters
	newCandidate := candidate[0:31] + candidate[len(candidate)-32:]
	return newCandidate

}
