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
	"strings"

	"github.com/gravitational/trace"
)

// QualifiedNameSeparator is the separator between scope and name in a
// scope-qualified name string. The colon character cannot appear in a valid
// scope segment (it does not match segmentRegexp), so "::" is an unambiguous
// separator for any strongly-validated scope.
const QualifiedNameSeparator = "::"

// QualifiedName pairs a scope with a resource name to uniquely identify a scoped
// resource. The canonical string form is "<scope>::<name>", e.g. "/staging/west::myrole".
type QualifiedName struct {
	// Scope is the resource's scope path, e.g. "/staging/west".
	Scope string
	// Name is the resource's name within its scope, e.g. "myrole".
	Name string
}

// String returns encodes the string representation of the QualifiedName.
func (q QualifiedName) String() string {
	return q.Scope + QualifiedNameSeparator + q.Name
}

// ParseQualifiedName parses a scope-qualified name string into its scope and name
// components by splitting on the first occurrence of "::". Returns an error if the
// separator is absent or either component is empty. This function does not validate
// the format of the scope or name components; use [StrongValidateQualifiedName] or
// [WeakValidateQualifiedName] for validation.
func ParseQualifiedName(sqn string) (QualifiedName, error) {
	idx := strings.Index(sqn, QualifiedNameSeparator)
	if idx < 0 {
		return QualifiedName{}, trace.BadParameter("scope-qualified name %q is missing %q separator", sqn, QualifiedNameSeparator)
	}

	scope := sqn[:idx]
	name := sqn[idx+len(QualifiedNameSeparator):]

	if scope == "" {
		return QualifiedName{}, trace.BadParameter("scope-qualified name %q has empty scope (root scope should be written as %q)", sqn, Root+QualifiedNameSeparator+name)
	}

	if name == "" {
		return QualifiedName{}, trace.BadParameter("scope-qualified name %q has empty name", sqn)
	}

	return QualifiedName{Scope: scope, Name: name}, nil
}

// StrongValidateQualifiedName validates a scope-qualified name using strong validation
// rules. This function *must* be called on all scope-qualified name values received from
// user input and/or cluster-external sources. Use [WeakValidateQualifiedName] when
// checking values from the control plane in logic that may run agent-side.
func StrongValidateQualifiedName(sqn string) error {
	qn, err := ParseQualifiedName(sqn)
	if err != nil {
		return trace.Wrap(err)
	}

	if err := StrongValidate(qn.Scope); err != nil {
		return trace.BadParameter("scope-qualified name %q has invalid scope: %v", sqn, err)
	}

	if err := StrongValidateSegment(qn.Name); err != nil {
		return trace.BadParameter("scope-qualified name %q has invalid name: %v", sqn, err)
	}

	// as an extra precaution, also run all weak checks just to be certain we didn't accidentally
	// construct a weak check that rejects something that would otherwise pass a strong check.
	if err := WeakValidateQualifiedName(sqn); err != nil {
		return trace.BadParameter("scope-qualified name would not pass weak validation: %v", err)
	}

	return nil
}

// WeakValidateQualifiedName performs a weak form of validation on a scope-qualified name.
// This is useful for ensuring that values received from trusted sources (e.g. the control
// plane) haven't been altered beyond our ability to reason effectively about them. Prefer
// [StrongValidateQualifiedName] for values received from external sources (e.g. user input).
func WeakValidateQualifiedName(sqn string) error {
	qn, err := ParseQualifiedName(sqn)
	if err != nil {
		return trace.Wrap(err)
	}

	if err := WeakValidate(qn.Scope); err != nil {
		return trace.BadParameter("scope-qualified name %q has invalid scope: %v", sqn, err)
	}

	if err := WeakValidateSegment(qn.Name); err != nil {
		return trace.BadParameter("scope-qualified name %q has invalid name: %v", sqn, err)
	}

	return nil
}
