package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRunInstallsThenChecks(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.Error(t, run(dir, true), "-check fails before the install")
	require.NoError(t, run(dir, false))
	require.NoError(t, run(dir, true), "-check passes after it")
}
