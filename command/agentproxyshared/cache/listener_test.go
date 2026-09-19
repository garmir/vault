// Copyright IBM Corp. 2016, 2025
// SPDX-License-Identifier: BUSL-1.1

package cache

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hashicorp/vault/internalshared/configutil"
	"github.com/stretchr/testify/require"
)

// TestStartListener_UnixSocketMode checks that socket_mode is applied to a
// unix listener on its own, without socket_user and socket_group also set.
func TestStartListener_UnixSocketMode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "agent.sock")

	bundle, err := StartListener(&configutil.Listener{
		Type:       "unix",
		Address:    path,
		SocketMode: "600",
		TLSDisable: true,
	})
	require.NoError(t, err)
	defer bundle.Listener.Close()

	fi, err := os.Stat(path)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o600), fi.Mode().Perm())
}
