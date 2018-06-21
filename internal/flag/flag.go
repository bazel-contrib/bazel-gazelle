// Copyright 2017 The Bazel Authors. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package flag

import (
	stdflag "flag"
	"strings"
)

// MultiFlag allows repeated string flags to be collected into a slice
type MultiFlag struct {
	Values *[]string
}

var _ stdflag.Value = (*MultiFlag)(nil)

func (m *MultiFlag) Set(v string) error {
	*m.Values = append(*m.Values, v)
	return nil
}

func (m *MultiFlag) String() string {
	if m == nil {
		return ""
	}
	return strings.Join(*m.Values, ",")
}

// ExplicitFlag is a string flag that tracks whether it was set.
type ExplicitFlag struct {
	IsSet *bool
	Value *string
}

var _ stdflag.Value = (*ExplicitFlag)(nil)

func (f *ExplicitFlag) Set(value string) error {
	*f.IsSet = true
	*f.Value = value
	return nil
}

func (f *ExplicitFlag) String() string {
	if f == nil {
		return ""
	}
	return *f.Value
}
