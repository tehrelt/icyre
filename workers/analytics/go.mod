module github.com/tehrelt/icyre/workers/analytics

go 1.26.0

toolchain go1.26.8

require (
	github.com/tehrelt/icyre/libs/contracts v0.0.0
	github.com/tehrelt/icyre/libs/platform v0.0.0-00010101000000-000000000000
)

require (
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	go.opentelemetry.io/otel v1.46.0 // indirect
	go.opentelemetry.io/otel/trace v1.46.0 // indirect
)

replace github.com/tehrelt/icyre/libs/platform => ../../libs/platform

replace github.com/tehrelt/icyre/libs/contracts => ../../libs/contracts
