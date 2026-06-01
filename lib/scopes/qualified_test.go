/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package scopes

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestQualifiedNameRoundTrip verifies basic encoding/decoding of scope-qualified names.
func TestQualifiedNameRoundTrip(t *testing.T) {
	t.Parallel()

	tts := []struct {
		name  string
		scope string
		rname string
		sqn   string
	}{
		{
			name:  "basic",
			scope: "/staging/west",
			rname: "myrole",
			sqn:   "/staging/west::myrole",
		},
		{
			name:  "single-segment scope",
			scope: "/staging",
			rname: "myrole",
			sqn:   "/staging::myrole",
		},
		{
			name:  "root scope",
			scope: "/",
			rname: "myrole",
			sqn:   "/::myrole",
		},
		{
			name:  "hyphenated name",
			scope: "/staging/west",
			rname: "my-role",
			sqn:   "/staging/west::my-role",
		},
		{
			name:  "uuid name",
			scope: "/staging",
			rname: "318ea8be-129c-41f4-ad95-fd830e14e3e7",
			sqn:   "/staging::318ea8be-129c-41f4-ad95-fd830e14e3e7",
		},
	}

	for _, tt := range tts {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// verify that encoding produces the expected sqn
			require.Equal(t, tt.sqn, QualifiedName{Scope: tt.scope, Name: tt.rname}.String())

			// verify that decoding produces the expected components
			qn, err := ParseQualifiedName(tt.sqn)
			require.NoError(t, err)
			require.Equal(t, tt.scope, qn.Scope)
			require.Equal(t, tt.rname, qn.Name)
		})
	}
}

// TestParseQualifiedNameErrors verifies expected error cases for ParseQualifiedName.
func TestParseQualifiedNameErrors(t *testing.T) {
	t.Parallel()

	tts := []struct {
		name string
		sqn  string
	}{
		{
			name: "no separator",
			sqn:  "staging-west-myrole",
		},
		{
			name: "empty scope",
			sqn:  "::myrole",
		},
		{
			name: "empty name",
			sqn:  "/staging/west::",
		},
		{
			name: "empty string",
			sqn:  "",
		},
		{
			name: "separator only",
			sqn:  "::",
		},
	}

	for _, tt := range tts {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := ParseQualifiedName(tt.sqn)
			require.Error(t, err)
		})
	}
}

func TestParseQualifiedNameMultipleSeparators(t *testing.T) {
	t.Parallel()

	qn, err := ParseQualifiedName("/staging::my::role")
	require.NoError(t, err)
	require.Equal(t, "/staging", qn.Scope)
	require.Equal(t, "my::role", qn.Name)

	// strong validation rejects the result since ":" is not in segmentRegexp
	require.Error(t, StrongValidateQualifiedName("/staging::my::role"))
	// weak validation passes — ":" is not in breakingChars, so WeakValidateSegment
	// accepts "my::role". This is the gap documented in the qualifiedSeparator comment.
	require.NoError(t, WeakValidateQualifiedName("/staging::my::role"))
}

func TestValidateQualifiedName(t *testing.T) {
	t.Parallel()

	tts := []struct {
		name     string
		sqn      string
		strongOk bool
		weakOk   bool
	}{
		{
			name:     "basic valid",
			sqn:      "/staging/west::myrole",
			strongOk: true,
			weakOk:   true,
		},
		{
			name:     "root scope",
			sqn:      "/::myrole",
			strongOk: true,
			weakOk:   true,
		},
		{
			name:     "single-segment scope",
			sqn:      "/staging::myrole",
			strongOk: true,
			weakOk:   true,
		},
		{
			name:     "hyphenated name",
			sqn:      "/staging::my-role",
			strongOk: true,
			weakOk:   true,
		},
		{
			name:     "uuid name",
			sqn:      "/staging::318ea8be-129c-41f4-ad95-fd830e14e3e7",
			strongOk: true,
			weakOk:   true,
		},
		{
			name:     "scope with breaking char",
			sqn:      "/stag@ing::myrole",
			strongOk: false,
			weakOk:   false,
		},
		{
			name:     "scope with space",
			sqn:      "/stag ing::myrole",
			strongOk: false,
			weakOk:   false,
		},
		{
			name:     "uppercase scope segment",
			sqn:      "/Staging/west::myrole",
			strongOk: false,
			weakOk:   true,
		},
		{
			name:     "scope segment too short",
			sqn:      "/a/west::myrole",
			strongOk: false,
			weakOk:   true,
		},
		{
			name:     "missing leading slash in scope",
			sqn:      "staging::myrole",
			strongOk: false,
			weakOk:   true,
		},
		{
			name:     "name with breaking char",
			sqn:      "/staging::my@role",
			strongOk: false,
			weakOk:   false,
		},
		{
			name:     "name with whitespace",
			sqn:      "/staging::my role",
			strongOk: false,
			weakOk:   false,
		},
		{
			name:     "name with slash",
			sqn:      "/staging::my/role",
			strongOk: false,
			weakOk:   false,
		},
		{
			name:     "uppercase name",
			sqn:      "/staging::MyRole",
			strongOk: false,
			weakOk:   true,
		},
		{
			name:     "name too short",
			sqn:      "/staging::x",
			strongOk: false,
			weakOk:   true,
		},
		{
			name:     "no separator",
			sqn:      "/staging-myrole",
			strongOk: false,
			weakOk:   false,
		},
		{
			name:     "empty scope component",
			sqn:      "::myrole",
			strongOk: false,
			weakOk:   false,
		},
		{
			name:     "empty name component",
			sqn:      "/staging::",
			strongOk: false,
			weakOk:   false,
		},
	}

	for _, tt := range tts {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := StrongValidateQualifiedName(tt.sqn)
			if tt.strongOk {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}

			err = WeakValidateQualifiedName(tt.sqn)
			if tt.weakOk {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}
}
