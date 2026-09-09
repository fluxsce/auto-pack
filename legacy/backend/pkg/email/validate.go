package email

import (
	"fmt"
	"net/mail"
	"strings"
)

// ValidateAddress checks a single RFC 5322 address.
func ValidateAddress(addr string) error {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return fmt.Errorf("email address is empty")
	}
	if _, err := mail.ParseAddress(addr); err != nil {
		return err
	}
	return nil
}

// ValidateAddresses checks each address in the slice.
func ValidateAddresses(addrs []string) error {
	for _, a := range addrs {
		if err := ValidateAddress(a); err != nil {
			return err
		}
	}
	return nil
}
