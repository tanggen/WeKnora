package utils

import (
	"strings"
)

// AccountType represents the kind of account identifier supplied during login.
type AccountType int

const (
	AccountTypeUnknown AccountType = iota
	AccountTypeEmail
	AccountTypePhone
	AccountTypeUsername
)

// ClassifyAccount determines the account type from the raw input string.
// Rules:
//   - Contains "@" → email
//   - All digits AND exactly 11 characters → phone
//   - Otherwise → username
func ClassifyAccount(account string) AccountType {
	if strings.Contains(account, "@") {
		return AccountTypeEmail
	}
	// Phone: exactly 11 digits
	if len(account) == 11 && isAllDigits(account) {
		return AccountTypePhone
	}
	return AccountTypeUsername
}

func isAllDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
