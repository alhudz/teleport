package main

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_convertToTerraform(t *testing.T) {
	type testCase struct {
		description string
		input       string
		expected    string
	}

	cases := []testCase{
		{
			description: "simple role",
			input: `kind: role
version: v7
metadata:
  name: manager
spec:
  allow:
    rules:
      - resources: ['user', 'role']
        verbs: ['list','read']
      - resources: ['session', 'event']
        verbs: ['list', 'read']
`,
			expected: `resource "teleport_role" "manager" {
  version = "v7"
  metadata = {
    name = "manager"
  }

  spec = {
    allow = {
      rules = [
        {
          resources = ["user", "role"]
          verbs     = ["list", "read"]
        },
        {
          resources = ["session", "event"]
          verbs     = ["list", "read"]
        }
      ]
    }
  }
}`,
		},
	}

	for _, c := range cases {
		t.Run(c.description, func(t *testing.T) {
			actual, err := convertToTerraform(strings.NewReader(c.input))
			assert.NoError(t, err)
			assert.Equal(t, c.expected, actual)
		})
	}
}
