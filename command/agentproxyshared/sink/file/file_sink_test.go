// Copyright IBM Corp. 2016, 2025
// SPDX-License-Identifier: BUSL-1.1

package file

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"

	hclog "github.com/hashicorp/go-hclog"
	uuid "github.com/hashicorp/go-uuid"
	"github.com/hashicorp/vault/command/agentproxyshared/sink"
	"github.com/hashicorp/vault/sdk/helper/logging"
	"github.com/stretchr/testify/require"
)

func testFileSink(t *testing.T, log hclog.Logger) (*sink.SinkConfig, string) {
	tmpDir := t.TempDir()

	path := filepath.Join(tmpDir, "token")

	config := &sink.SinkConfig{
		Logger: log.Named("sink.file"),
		Config: map[string]interface{}{
			"path": path,
		},
	}

	s, err := NewFileSink(config)
	if err != nil {
		t.Fatal(err)
	}
	config.Sink = s

	return config, tmpDir
}

func TestFileSink(t *testing.T) {
	log := logging.NewVaultLogger(hclog.Trace)

	fs, tmpDir := testFileSink(t, log)
	defer os.RemoveAll(tmpDir)

	path := filepath.Join(tmpDir, "token")

	uuidStr, _ := uuid.GenerateUUID()
	if err := fs.WriteToken(uuidStr); err != nil {
		t.Fatal(err)
	}

	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}

	fi, err := file.Stat()
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode() != os.FileMode(0o640) {
		t.Fatalf("wrong file mode was detected at %s", path)
	}
	err = file.Close()
	if err != nil {
		t.Fatal(err)
	}

	fileBytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	if string(fileBytes) != uuidStr {
		t.Fatalf("expected %s, got %s", uuidStr, string(fileBytes))
	}
}

func testFileSinkMode(t *testing.T, log hclog.Logger, gid int) (*sink.SinkConfig, string) {
	tmpDir := t.TempDir()

	path := filepath.Join(tmpDir, "token")

	config := &sink.SinkConfig{
		Logger: log.Named("sink.file"),
		Config: map[string]interface{}{
			"path":  path,
			"mode":  0o644,
			"group": gid,
		},
	}

	s, err := NewFileSink(config)
	if err != nil {
		t.Fatal(err)
	}
	config.Sink = s

	return config, tmpDir
}

func TestFileSinkMode(t *testing.T) {
	log := logging.NewVaultLogger(hclog.Trace)

	fs, tmpDir := testFileSinkMode(t, log, os.Getegid())
	defer os.RemoveAll(tmpDir)

	path := filepath.Join(tmpDir, "token")

	uuidStr, _ := uuid.GenerateUUID()
	if err := fs.WriteToken(uuidStr); err != nil {
		t.Fatal(err)
	}

	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	fi, err := file.Stat()
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode() != os.FileMode(0o644) {
		t.Fatalf("wrong file mode was detected at %s", path)
	}

	fileBytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	if string(fileBytes) != uuidStr {
		t.Fatalf("expected %s, got %s", uuidStr, string(fileBytes))
	}
}

// TestFileSinkMode_Ownership tests that the file is owned by the group specified
// in the configuration. This test requires the current user to be in at least two
// groups. If the user is not in two groups, the test will be skipped.
func TestFileSinkMode_Ownership(t *testing.T) {
	groups, err := os.Getgroups()
	if err != nil {
		t.Fatal(err)
	}

	if len(groups) < 2 {
		t.Skip("not enough groups to test file ownership")
	}

	// find a group that is not the current group
	var gid int
	for _, g := range groups {
		if g != os.Getegid() {
			gid = g
			break
		}
	}

	log := logging.NewVaultLogger(hclog.Trace)

	fs, tmpDir := testFileSinkMode(t, log, gid)
	defer os.RemoveAll(tmpDir)

	path := filepath.Join(tmpDir, "token")

	uuidStr, _ := uuid.GenerateUUID()
	if err := fs.WriteToken(uuidStr); err != nil {
		t.Fatal(err)
	}

	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	fi, err := file.Stat()
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode() != os.FileMode(0o644) {
		t.Fatalf("wrong file mode was detected at %s", path)
	}
	// check if file is owned by the group
	if fi.Sys().(*syscall.Stat_t).Gid != uint32(gid) {
		t.Fatalf("file is not owned by the group %d", gid)
	}

	fileBytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	if string(fileBytes) != uuidStr {
		t.Fatalf("expected %s, got %s", uuidStr, string(fileBytes))
	}
}

// TestFileSinkMode_Parsing checks how the mode option is read: a number is
// taken as is and must be an octal literal in hcl, a string of octal digits
// is accepted for json configs, and a number with bits above the
// permission bits is rejected since it is almost always a decimal value
// written where an octal one was meant.
func TestFileSinkMode_Parsing(t *testing.T) {
	log := logging.NewVaultLogger(hclog.Trace)

	tests := []struct {
		name     string
		mode     interface{}
		wantMode os.FileMode
		wantErr  string
	}{
		{name: "octal literal", mode: 0o640, wantMode: 0o640},
		{name: "string with leading zero", mode: "0640", wantMode: 0o640},
		{name: "string without leading zero", mode: "640", wantMode: 0o640},
		{name: "decimal with bits above 0777", mode: 640, wantErr: "octal"},
		{name: "string that is not octal", mode: "0689", wantErr: "octal"},
		{name: "unsupported type", mode: 6.4, wantErr: "'mode'"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "token")
			config := &sink.SinkConfig{
				Logger: log.Named("sink.file"),
				Config: map[string]interface{}{
					"path": path,
					"mode": tc.mode,
				},
			}

			s, err := NewFileSink(config)
			if tc.wantErr != "" {
				require.ErrorContains(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)

			require.NoError(t, s.WriteToken("token"))
			fi, err := os.Stat(path)
			require.NoError(t, err)
			require.Equal(t, tc.wantMode, fi.Mode().Perm())
		})
	}
}
