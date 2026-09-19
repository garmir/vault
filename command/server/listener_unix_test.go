// Copyright IBM Corp. 2016, 2025
// SPDX-License-Identifier: BUSL-1.1

package server

import (
	"net"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/hashicorp/cli"
	"github.com/hashicorp/vault/internalshared/configutil"
	"github.com/stretchr/testify/require"
)

func TestUnixListener(t *testing.T) {
	ln, _, _, err := unixListenerFactory(&configutil.Listener{
		Address: filepath.Join(t.TempDir(), "/vault.sock"),
	}, nil, cli.NewMockUi())
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	connFn := func(lnReal net.Listener) (net.Conn, error) {
		return net.Dial("unix", ln.Addr().String())
	}

	testListenerImpl(t, ln, connFn, "", 0, "", false)
}

// TestUnixListenerFactory_SocketOptions checks that the socket permission
// options are applied when only some of them are set, since each one is
// optional on its own.
func TestUnixListenerFactory_SocketOptions(t *testing.T) {
	tests := []struct {
		name     string
		listener configutil.Listener
		wantMode os.FileMode
	}{
		{
			name:     "mode only",
			listener: configutil.Listener{SocketMode: "600"},
			wantMode: 0o600,
		},
		{
			name:     "mode and group",
			listener: configutil.Listener{SocketMode: "660", SocketGroup: strconv.Itoa(os.Getgid())},
			wantMode: 0o660,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tc.listener.Address = filepath.Join(t.TempDir(), "vault.sock")

			ln, _, _, err := unixListenerFactory(&tc.listener, nil, nil)
			require.NoError(t, err)
			defer ln.Close()

			fi, err := os.Stat(tc.listener.Address)
			require.NoError(t, err)
			require.Equal(t, tc.wantMode, fi.Mode().Perm())
		})
	}
}
