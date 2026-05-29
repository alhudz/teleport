package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/tfgen"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
)

func Test_convertYAMLToHCL(t *testing.T) {
	type testCase struct {
		description string
		input       string
		expected    string
	}
	conf := map[string]jsonToHCLConverter{
		"role": func(data []byte) (tfgen.Resource, error) {
			var role types.RoleV6
			if err := utils.FastUnmarshal(data, &role); err != nil {
				return nil, trace.Errorf("invalid Teleport role in the input %w", err)
			}
			return &role, nil
		},
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
      rules = [{
        resources = ["user", "role"]
        verbs     = ["list", "read"]
        }, {
        resources = ["session", "event"]
        verbs     = ["list", "read"]
      }]
    }
  }
}
`,
		},
	}

	for _, c := range cases {
		t.Run(c.description, func(t *testing.T) {
			var buf bytes.Buffer
			err := convertYAMLToHCL(&buf, strings.NewReader(c.input), conf)
			assert.NoError(t, err)
			assert.Equal(t, c.expected, buf.String())
		})
	}
}
