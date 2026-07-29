package command

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMultiWordTeamNameFromFields(t *testing.T) {
	fields := strings.Fields("/community-admin remove-from-team child.example U12 Soccer Team")
	assert.Equal(t, "U12 Soccer Team", strings.Join(fields[3:], " "))
}
