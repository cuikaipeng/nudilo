//go:build e2e

package desktop

import "os"

func init() {
	// Instrumented test builds must not activate an installed user's app.
	if root := os.Getenv("CALLBELL_DATA_DIR"); root != "" {
		singleInstanceKey = "Nudilo-E2E-" + root
	}
}
