package handlers

import (
	"net/mail"
	"strings"
)

func validateEmail(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "is required"
	}
	if _, err := mail.ParseAddress(s); err != nil {
		return "must be a valid email"
	}
	return ""
}

func validatePassword(s string) string {
	if len(s) < 8 {
		return "must be at least 8 characters"
	}
	return ""
}

func validateRequired(field, s string) string {
	if strings.TrimSpace(s) == "" {
		return "is required"
	}
	return ""
}
