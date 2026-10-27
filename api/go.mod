module github.com/catalystcommunity/catalystcal/api

go 1.26.4

replace github.com/catalystcommunity/catalystcal/coredb => ../coredb

require (
	github.com/catalystcommunity/catalystcal/coredb v0.0.0-00010101000000-000000000000
	github.com/catalystcommunity/linkkeys/sdks/regular-rp/go v0.0.0-20261012235123-03fcbd8e7007
	github.com/emersion/go-ical v0.0.0-20250609112844-439c63cef608
	github.com/fxamacker/cbor/v2 v2.9.2
	github.com/oklog/ulid/v2 v2.1.2
	github.com/pressly/goose/v3 v3.24.1
	github.com/rs/cors v1.11.1
	github.com/sirupsen/logrus v1.9.3
	github.com/teambition/rrule-go v1.8.2
	modernc.org/sqlite v1.60.1
)

require (
	github.com/catalystcommunity/csilgen/transports/go v0.0.0-20260713013116-a661c8727022 // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/mattn/go-isatty v0.0.24 // indirect
	github.com/mfridman/interpolate v0.0.2 // indirect
	github.com/ncruces/go-strftime v1.0.0 // indirect
	github.com/remyoudompheng/bigfft v0.0.0-20230129092748-24d4a6f8daec // indirect
	github.com/sethvargo/go-retry v0.3.0 // indirect
	github.com/x448/float16 v0.8.4 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/crypto v0.54.0 // indirect
	golang.org/x/sync v0.23.0 // indirect
	golang.org/x/sys v0.48.0 // indirect
	modernc.org/libc v1.77.1 // indirect
	modernc.org/mathutil v1.7.1 // indirect
	modernc.org/memory v1.12.1 // indirect
)
