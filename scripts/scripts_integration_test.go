//go:build integration
// +build integration

package scripts

import (
	"os"
	"testing"
)

func TestScriptsIntegration(t *testing.T) {
	if os.Getenv("RUN_SCRIPTS_TESTS") == "" {
		t.Skip("skipping integration test; set RUN_SCRIPTS_TESTS=1 to run")
	}

	soCrypto := "build/artifacts/artifacts/sqlean/crypto.so"

	t.Run("CallExtDebug", func(t *testing.T) {
		RunCallExtDebug()
	})

	t.Run("DebugLoadExt", func(t *testing.T) {
		RunDebugLoadExt(soCrypto)
	})

	t.Run("SmokeLibSQL", func(t *testing.T) {
		RunSmokeLibSQL()
	})

	t.Run("SqleanProbe", func(t *testing.T) {
		RunSqleanProbe()
	})

	t.Run("TryLoadExtension", func(t *testing.T) {
		RunTryLoadExtension()
	})
}
