package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	convertv1 "github.com/gravitational/teleport/api/types/accesslist/convert/v1"
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
				return nil, trace.Errorf("invalid Teleport role: %w", err)
			}
			return &role, nil
		},
		"access_list": func(data []byte) (tfgen.Resource, error) {
			// Unmarshal to accesslist.AccessList to apply custom
			// unmarshalers.
			var al accesslist.AccessList
			if err := utils.FastUnmarshal(data, &al); err != nil {
				return nil, trace.Errorf("invalid access_list: %w", err)

			}

			// Convert to the proto type, which tfgen requires
			proto := convertv1.ToProto(&al)

			// Wrap to implement the Resource interface
			return tfgen.WrapHeaderResource(proto), nil
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
		{
			description: "access list",
			input: `version: v1
kind: access_list
metadata:
  name: support-engineers
spec:
  title: "Production access for support engineers"
  audit:
    recurrence:
      frequency: 6months
  description: "Use this Access List to grant access to production to your engineers enrolled in the
support rotation."
  owners:
    - description: "manager of NA support team"
      name: alice
  ownership_requires:
    roles:
      - manager
  grants:
    roles:
      - support-engineer
  membership_requires:
    roles:
      - engineer
`,
			expected: `resource "teleport_access_list" "support-engineers" {
  header = {
    kind    = "access_list"
    version = "v1"
    metadata = {
      name = "support-engineers"
    }
  }

  spec = {
    description = "Use this Access List to grant access to production to your engineers enrolled in the support rotation."
    owners = [{
      description       = "manager of NA support team"
      ineligible_status = "0"
      membership_kind   = "0"
      name              = "alice"
    }]
    audit = {
      recurrence = {
        frequency    = "6"
        day_of_month = "0"
      }
    }
    membership_requires = {
      roles = ["engineer"]
    }
    ownership_requires = {
      roles = ["manager"]
    }
    grants = {
      roles = ["support-engineer"]
    }
    title = "Production access for support engineers"
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
