/* Copyright 2026 The Bazel Authors. All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

   http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package errors

import "fmt"

// Severity indicates how serious an error is. Gazelle may stop early
// skip writing BUILD files, or exit non-zero based on error severity.
type Severity int

const (
	// Error indicates something is wrong, but Gazelle may be able to continue
	// working, though it may produce incorrect BUILD files. For example, this
	// might be a source file parse error or unresolved import. Error is the
	// default level for errors that don't express severity.
	Error Severity = 0

	// Warning indicates something is wrong, but Gazelle can continue working
	// and will produce correct BUILD files. Gazelle exits with a successful
	// status, even in strict mode. For example, this might be emitted
	// for reliance on a deprecated feature.
	Warning Severity = 1

	// Critical indicates something is so wrong that Gazelle cannot continue
	// working. Gazelle exits with a failing status, even when strict mode
	// is disabled. For example, this might be emitted when an extension's
	// run-time dependency is missing.
	Critical Severity = 2
)

func (s Severity) String() string {
	switch s {
	case Error:
		return "error"
	case Warning:
		return "warning"
	case Critical:
		return "critical"
	default:
		return "unknown"
	}
}

// SeverityError wraps an error, indicating its severity level.
type SeverityError struct {
	Severity Severity
	Err      error
}

// WithSeverity wraps an error with a given severity level.
func WithSeverity(sev Severity, err error) error {
	return &SeverityError{Severity: sev, Err: err}
}

// SeverityErrorf formats a new error with a given severity level.
func SeverityErrorf(sev Severity, format string, args ...any) error {
	return WithSeverity(sev, fmt.Errorf(format, args...))
}

func (e *SeverityError) Error() string {
	return e.Err.Error()
}

func (e *SeverityError) Unwrap() error {
	return e.Err
}
