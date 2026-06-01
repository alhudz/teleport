package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_convertYAMLToHCL(t *testing.T) {
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
		{
			description: "rfd 153 resource",
			input: `kind: bot
version: v1
metadata:
  name: example
spec:
  roles:
  - editor
  traits:
  - name: logins
    values:
    - root
`,
			expected: `resource "teleport_bot" "example" {
  version = "v1"

  metadata = {
    name = "example"
  }

  spec = {
    roles = ["editor"]
    traits = [{
      name   = "logins"
      values = ["root"]
    }]
  }
}
`,
		},
	}
	for _, c := range cases {
		t.Run(c.description, func(t *testing.T) {
			var buf bytes.Buffer
			err := convertYAMLToHCL(&buf, strings.NewReader(c.input))
			assert.NoError(t, err)
			assert.Equal(t, c.expected, buf.String())
		})
	}
}

func Test_convertYAMLToKubernetes(t *testing.T) {
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
			expected: `apiVersion: resources.teleport.dev/v1
kind: TeleportRoleV8
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
		},
		// 		{
		// 			description: "access list",
		// 			input: `version: v1
		// kind: access_list
		// metadata:
		//   name: support-engineers
		// spec:
		//   title: "Production access for support engineers"
		//   audit:
		//     recurrence:
		//       frequency: 6months
		//   description: "Use this Access List to grant access to production to your engineers enrolled in the
		// support rotation."
		//   owners:
		//     - description: "manager of NA support team"
		//       name: alice
		//   ownership_requires:
		//     roles:
		//       - manager
		//   grants:
		//     roles:
		//       - support-engineer
		//   membership_requires:
		//     roles:
		//       - engineer
		// `,
		// 			expected: ``,
		// 		},
		//		{
		//			description: "rfd 153 resource",
		//			input: `kind: bot
		//version: v1
		//metadata:
		//  name: example
		//spec:
		//  roles:
		//  - editor
		//  traits:
		//  - name: logins
		//    values:
		//    - root
		//`,
		//			expected: ``,
		//		},
	}
	for _, c := range cases {
		t.Run(c.description, func(t *testing.T) {
			var buf bytes.Buffer
			err := convertYAMLtoKubernetes(&buf, strings.NewReader(c.input))
			assert.NoError(t, err)
			assert.Equal(t, c.expected, buf.String())
		})
	}
}
