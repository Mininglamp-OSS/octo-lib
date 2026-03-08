package util

// base62Chars maps values 0-61 to their base62 character.
// 0-9: '0'-'9', 10-35: 'a'-'z', 36-61: 'A'-'Z'
const base62Chars = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

// Ten2Hex converts a decimal number to base62 string.
func Ten2Hex(ten int64) string {
	if ten == 0 {
		return "0"
	}
	hex := ""
	for ten > 0 {
		remainder := ten % 62
		hex = string(base62Chars[remainder]) + hex
		ten = ten / 62
	}
	return hex
}
