package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

func TestValidateUsername(t *testing.T) {
	require.NoError(t, ValidateUsername("child.example"))
	require.Error(t, ValidateUsername("BadUser"))
	require.Error(t, ValidateUsername("has space"))
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
