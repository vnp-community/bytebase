package db

import (
	storepb "github.com/bytebase/bytebase/backend/generated-go/store"
)

// RegisteredEngines returns a list of all database engines that have a registered driver.
// This is useful for runtime discovery of available engines, especially when
// build tags are used to include/exclude specific drivers.
func RegisteredEngines() []storepb.Engine {
	driversMu.RLock()
	defer driversMu.RUnlock()

	engines := make([]storepb.Engine, 0, len(drivers))
	for engine := range drivers {
		engines = append(engines, engine)
	}
	return engines
}

// IsEngineRegistered checks if a specific engine has a registered driver.
func IsEngineRegistered(engine storepb.Engine) bool {
	driversMu.RLock()
	defer driversMu.RUnlock()
	_, ok := drivers[engine]
	return ok
}

// RegisteredEngineCount returns the number of registered database engines.
func RegisteredEngineCount() int {
	driversMu.RLock()
	defer driversMu.RUnlock()
	return len(drivers)
}
