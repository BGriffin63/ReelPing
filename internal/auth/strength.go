package auth

import (
	"strings"
	"unicode"
)

// MinPasswordLength is the enforced minimum administrator password length.
const MinPasswordLength = 12

// commonPasswords is a small blocklist of trivially weak passwords. It is not
// exhaustive; it exists to reject the most obvious choices outright.
var commonPasswords = map[string]bool{
	"password": true, "password1": true, "password123": true,
	"12345678": true, "123456789": true, "1234567890": true,
	"qwertyuiop": true, "qwerty123": true, "administrator": true,
	"changeme": true, "letmein123": true, "welcome123": true,
	"reelping": true, "reelping123": true, "plexplexplex": true,
	"iloveyou": true, "adminadmin": true, "passw0rd": true,
}

// PasswordStrength returns a score 0..4 and human guidance.
type PasswordStrength struct {
	Score      int    // 0 (weak) .. 4 (strong)
	Label      string // "Very weak".."Strong"
	Acceptable bool
	Reason     string // why it is not acceptable (empty if acceptable)
}

// CheckPassword validates an administrator password against the minimum policy
// and returns strength feedback.
func CheckPassword(pw string) PasswordStrength {
	if len(pw) < MinPasswordLength {
		return PasswordStrength{Score: 0, Label: "Too short", Acceptable: false,
			Reason: "Password must be at least 12 characters."}
	}
	if commonPasswords[strings.ToLower(pw)] {
		return PasswordStrength{Score: 0, Label: "Too common", Acceptable: false,
			Reason: "That password is too common. Choose something unique."}
	}

	var hasLower, hasUpper, hasDigit, hasSymbol bool
	for _, r := range pw {
		switch {
		case unicode.IsLower(r):
			hasLower = true
		case unicode.IsUpper(r):
			hasUpper = true
		case unicode.IsDigit(r):
			hasDigit = true
		case unicode.IsPunct(r) || unicode.IsSymbol(r):
			hasSymbol = true
		}
	}
	classes := 0
	for _, b := range []bool{hasLower, hasUpper, hasDigit, hasSymbol} {
		if b {
			classes++
		}
	}

	score := 1
	if len(pw) >= 16 {
		score++
	}
	if len(pw) >= 24 {
		score++
	}
	if classes >= 3 {
		score++
	}
	if score > 4 {
		score = 4
	}

	labels := []string{"Very weak", "Weak", "Fair", "Good", "Strong"}
	ps := PasswordStrength{Score: score, Label: labels[score], Acceptable: true}
	// A 12+ char passphrase with low class-count is still acceptable (length
	// beats complexity), matching modern guidance.
	if len(pw) < 16 && classes < 2 {
		ps.Acceptable = false
		ps.Reason = "Add more length or a mix of character types."
		if ps.Score > 1 {
			ps.Score = 1
			ps.Label = labels[1]
		}
	}
	return ps
}
