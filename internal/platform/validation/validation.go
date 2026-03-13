package validation

import (
	"net/mail"
	"strings"
)

func IsEmail(value string) bool {
	address, err := mail.ParseAddress(value)
	if err != nil {
		return false
	}

	return strings.EqualFold(address.Address, value)
}
