package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

func TestValidateUsername(t *testing.T) {
	tests := []struct {
		name     string
		username string
		valid    bool
	}{
		{name: "valid", username: "child.example", valid: true},
		{name: "valid with digits and punctuation", username: "child2._-example", valid: true},
		{name: "uppercase", username: "BadUser"},
		{name: "space", username: "has space"},
		{name: "leading password flag", username: "--password"},
		{name: "leading help flag", username: "-h"},
		{name: "only flags", username: "--"},
		{name: "leading digit", username: "1child"},
		{name: "empty", username: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateUsername(tt.username)
			if tt.valid {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}
}

func TestGeneratePassword(t *testing.T) {
	pw, err := GeneratePassword()
	require.NoError(t, err)
	assert.Len(t, pw, 16)
	assert.Regexp(t, `[A-Z]`, pw)
	assert.Regexp(t, `[a-z]`, pw)
	assert.Regexp(t, `[0-9]`, pw)
	assert.Regexp(t, `[!@#$%&*]`, pw)

	seen := map[string]bool{}
	for range 20 {
		p, err := GeneratePassword()
		require.NoError(t, err)
		seen[p] = true
	}
	assert.GreaterOrEqual(t, len(seen), 18)
}

func TestHashPassword(t *testing.T) {
	hash, err := HashPassword("SecretPass1!")
	require.NoError(t, err)
	require.NoError(t, bcrypt.CompareHashAndPassword([]byte(hash), []byte("SecretPass1!")))
}
