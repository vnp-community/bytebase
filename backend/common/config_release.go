//go:build release

// AI-CONTEXT: Runtime Mode = "release" (production)
// AI-CONTEXT: This file is compiled ONLY when: -tags release
// AI-CONTEXT: IsDev() returns false — disables debug logging, enforces strict security.

//nolint:revive
package common

func IsDev() bool {
	return false
}
