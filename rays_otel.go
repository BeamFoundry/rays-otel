// # Initialization
//
// This package should be imported from a main file alongside rays:
//
//	package main
//
//	import (
//	  "github.com/BeamFoundry/rays"
//	  _ "github.com/BeamFoundry/rays-otel"
//	)
//
// # Packages
//
// This project divides its code between the following packages:
//
//   - [github.com/BeamFoundry/rays-otel/pkg/core]: Contains configuration properties,
//     customizer interfaces, and slog/OpenTelemetry logging handlers.
package rays_otel //nolint:revive // Ignoring package naming hint

import (
	"embed"

	"github.com/BeamFoundry/rays"

	_ "github.com/BeamFoundry/rays-otel/internal/autoconfig" // autoconfig
	_ "github.com/BeamFoundry/rays-otel/pkg/core"            // core
)

//go:embed config/*.yaml
var config embed.FS

func init() {
	//nolint:errcheck // Default embedded config FS cannot fail
	rays.ApplicationContext().AddDefaultConfigFS(config)
}
