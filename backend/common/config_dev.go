//go:build !release

// AI-CONTEXT: Runtime Mode = "development" (default)
// AI-CONTEXT: This file is compiled when the "release" build tag is NOT set.
// AI-CONTEXT: IsDev() returns true — enables debug logging, relaxed security, demo data.

//nolint:revive
package common

func IsDev() bool {
	return true
}
