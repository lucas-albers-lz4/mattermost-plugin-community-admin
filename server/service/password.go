package service

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"regexp"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const (
	usernamePattern = `^[a-z0-9._-]+$`
	upperChars      = "ABCDEFGHJKLMNPQRSTUVWXYZ"
	lowerChars      = "abcdefghjkmnpqrstuvwxyz"
	digitChars      = "23456789"
	symbolChars     = "!@#$%&*"
)

var usernameRe = regexp.MustCompile(usernamePattern)

// ValidateUsername checks community username rules.
func ValidateUsername(username string) error {
	if !usernameRe.MatchString(username) {
		return fmt.Errorf("invalid username (use lowercase letters, digits, . _ -): %s", username)
	}
	return nil
}

// GeneratePassword creates a 16-character password meeting community policy.
func GeneratePassword() (string, error) {
	required := []string{
		randomChar(upperChars),
		randomChar(lowerChars),
		randomChar(digitChars),
		randomChar(symbolChars),
	}
	all := upperChars + lowerChars + digitChars + symbolChars
	for range 12 {
		required = append(required, randomChar(all))
	}
	if err := cryptoShuffle(required); err != nil {
		return "", err
	}
	return strings.Join(required, ""), nil
}

// HashPassword returns a bcrypt hash suitable for mmctl --hashed.
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

func cryptoShuffle(items []string) error {
	for i := len(items) - 1; i > 0; i-- {
		jBig, err := rand.Int(rand.Reader, big.NewInt(int64(i+1)))
		if err != nil {
			return err
		}
		j := int(jBig.Int64())
		items[i], items[j] = items[j], items[i]
	}
	return nil
}

func randomChar(charset string) string {
	n, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
	if err != nil {
		return string(charset[0])
	}
	return string(charset[n.Int64()])
}

// ParentTextLine formats the SMS handoff line for parents.
func ParentTextLine(siteURL, username, password string) string {
	return fmt.Sprintf(
		"TEXT_TO_PARENT: Your community chat login — %s — username: %s — password: %s — save this message; contact the admin if you need a reset.",
		siteURL, username, password,
	)
}

// HourBucket returns the current hour bucket for rate limiting.
func HourBucket() string {
	return time.Now().UTC().Format("2006-01-02T15")
}
