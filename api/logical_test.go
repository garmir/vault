// Copyright IBM Corp. 2016, 2025
// SPDX-License-Identifier: MPL-2.0

package api

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLogical_addExtraHeaders(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		r           *Request
		headers     http.Header
		wantHeaders http.Header
		wantErr     assert.ErrorAssertionFunc
	}{
		{
			name: "nil headers",
			r: &Request{
				Headers: http.Header{
					"X-Other-Header": []string{"other-value"},
				},
			},
			headers: nil,
			wantHeaders: http.Header{
				"X-Other-Header": []string{"other-value"},
			},
			wantErr: assert.NoError,
		},
		{
			name: "empty headers",
			r: &Request{
				Headers: http.Header{
					"X-Other-Header": []string{"other-value"},
				},
			},
			headers: http.Header{},
			wantHeaders: http.Header{
				"X-Other-Header": []string{"other-value"},
			},
			wantErr: assert.NoError,
		},
		{
			name:    "no headers",
			r:       &Request{},
			wantErr: assert.NoError,
		},
		{
			name: "nil request",
			headers: http.Header{
				"X-Extra-Header": []string{"real-value"},
			},
			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool {
				return assert.ErrorContains(t, err, "nil request", i...)
			},
		},
		{
			name: "extra headers",
			r: &Request{
				Headers: http.Header{
					"X-Other-Header": []string{"other-value"},
				},
			},
			headers: http.Header{
				"X-Extra-Header": []string{"real-value"},
			},
			wantHeaders: http.Header{
				"X-Extra-Header": []string{"real-value"},
				"X-Other-Header": []string{"other-value"},
			},
			wantErr: assert.NoError,
		},
		{
			name: "reserved header",
			r: &Request{
				Headers: http.Header{
					"X-Reserved-Header": []string{"reserved-value"},
				},
			},
			headers: http.Header{
				"x-rESERved-hEAder": []string{"other-value"},
			},
			wantHeaders: http.Header{
				"X-Reserved-Header": []string{"reserved-value"},
			},
			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool {
				return assert.ErrorContains(t, err,
					fmt.Sprintf("cannot set extra header %q, it is reserved", "X-Reserved-Header"), i...)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Logical{}
			tt.wantErr(t, c.addExtraHeaders(tt.r, tt.headers), fmt.Sprintf("addExtraHeaders(%v, %v)", tt.r, tt.headers))
			if tt.r == nil {
				if tt.wantHeaders != nil {
					require.Fail(t, "invalid test case: nil request with headers", "addExtraHeaders(%v, %v)", tt.r, tt.headers)
				}
				return
			}
			assert.Equalf(t, tt.wantHeaders, tt.r.Headers, "Headers after addExtraHeaders(%v, %v)", tt.r, tt.headers)
		})
	}
}

// TestLogical_RawWriteBodyOutlivesCall checks that the raw write and patch
// helpers hand back a body that can still be read after the call returns.
// the server sends the body in two parts with a pause between them, so a
// context cancelled on return severs the read of the second part.
func TestLogical_RawWriteBodyOutlivesCall(t *testing.T) {
	handler := func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"data":{"first":"part",`))
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		time.Sleep(200 * time.Millisecond)
		w.Write([]byte(`"second":"part"}}`))
	}
	config, ln := testHTTPServer(t, http.HandlerFunc(handler))
	defer ln.Close()

	// a configured timeout is what installs the cancel that used to fire early
	config.Timeout = 30 * time.Second

	client, err := NewClient(config)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	client.SetToken("foo")

	calls := map[string]func(context.Context) (*Response, error){
		"write": func(ctx context.Context) (*Response, error) {
			return client.Logical().WriteRawWithContext(ctx, "secret/foo", []byte(`{"a":"b"}`))
		},
		"patch": func(ctx context.Context) (*Response, error) {
			return client.Logical().PatchRawWithContext(ctx, "secret/foo", []byte(`{"a":"b"}`))
		},
	}

	for name, call := range calls {
		t.Run(name, func(t *testing.T) {
			resp, err := call(context.Background())
			if err != nil {
				t.Fatalf("err: %s", err)
			}
			defer resp.Body.Close()

			body, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatalf("reading the body after the call returned: %s", err)
			}
			if want := `{"data":{"first":"part","second":"part"}}`; string(body) != want {
				t.Fatalf("body = %q, want %q", body, want)
			}
		})
	}
}
