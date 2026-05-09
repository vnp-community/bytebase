package server

import (
	storepb "github.com/bytebase/bytebase/backend/generated-go/store"
	"github.com/bytebase/bytebase/backend/plugin/db"
)

// DriverRegistry provides runtime discovery of available database engines.
// The concrete implementation delegates to the plugin/db registration system
// which is populated at init-time via build tags.
type DriverRegistry interface {
	AvailableEngines() []storepb.Engine
	IsEngineAvailable(engine storepb.Engine) bool
}

// runtimeRegistry implements DriverRegistry by querying the global driver registry.
type runtimeRegistry struct{}

// NewDriverRegistry returns a DriverRegistry that queries the compiled-in database drivers.
func NewDriverRegistry() DriverRegistry {
	return &runtimeRegistry{}
}

func (r *runtimeRegistry) AvailableEngines() []storepb.Engine {
	return db.RegisteredEngines()
}

func (r *runtimeRegistry) IsEngineAvailable(engine storepb.Engine) bool {
	return db.IsEngineRegistered(engine)
}
