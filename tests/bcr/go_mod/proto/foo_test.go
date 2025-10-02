package proto

import (
	"testing"

	"github.com/bazelbuild/bazel-gazelle/tests/bcr/proto/foo"
	"google.golang.org/genproto/googleapis/bytestream"
	"google.golang.org/genproto/protobuf/ptype"
	"google.golang.org/protobuf/types/known/sourcecontextpb"
	"google.golang.org/protobuf/types/known/timestamppb"
	"google.golang.org/protobuf/types/known/typepb"
)

func TestWellKnownTypes(t *testing.T) {
	var foo foo.Foo
	foo.Name = "foo"
	foo.Type = &typepb.Type{
		Name:          "my_type",
		SourceContext: &sourcecontextpb.SourceContext{},
	}
	foo.LastUpdated = &timestamppb.Timestamp{
		Seconds: 12345,
		Nanos:   67890,
	}
}

func TestSubModules(t *testing.T) {
	var bsr bytestream.ReadRequest
	bsr.ResourceName = "resource_name"
	var e ptype.Enum
	e.Name = "enum_name"
}
