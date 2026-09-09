module example.com/repro

go 1.24.12

require (
	example.com/thirdparty v0.0.0-00010101000000-000000000000
	example.com/repro/pkg/foo v0.0.0-00010101000000-000000000000 // indirect
)

replace (
	example.com/thirdparty => ./thirdparty
	example.com/repro/pkg/foo => ./pkg/foo
)
