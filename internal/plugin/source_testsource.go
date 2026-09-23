//go:build testsource

package plugin

import "os"

func testSourceOverride() string { return os.Getenv("MOLE_ZH_SOURCE") }
