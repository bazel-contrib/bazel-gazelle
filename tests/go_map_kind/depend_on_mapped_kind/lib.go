package depend_on_mapped_kind
import (
	_ "example.com/mapkind/disabled"
	_ "example.com/mapkind/enabled"
	_ "example.com/mapkind/enabled/existing_rules/mapped"
	_ "example.com/mapkind/enabled/existing_rules/unmapped"
	_ "example.com/mapkind/enabled/overridden"
)