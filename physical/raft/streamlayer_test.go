// Copyright IBM Corp. 2016, 2025
// SPDX-License-Identifier: BUSL-1.1

package raft

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"net"
	"testing"
	"time"

	"github.com/hashicorp/vault/vault/cluster"
)

type mockClusterHook struct {
	address net.Addr
}

func (*mockClusterHook) AddClient(alpn string, client cluster.Client)       {}
func (*mockClusterHook) RemoveClient(alpn string)                           {}
func (*mockClusterHook) AddHandler(alpn string, handler cluster.Handler)    {}
func (*mockClusterHook) StopHandler(alpn string)                            {}
func (*mockClusterHook) TLSConfig(ctx context.Context) (*tls.Config, error) { return nil, nil }
func (m *mockClusterHook) Addr() net.Addr                                   { return m.address }
func (*mockClusterHook) GetDialerFunc(ctx context.Context, alpnProto string) func(string, time.Duration) (net.Conn, error) {
	return func(string, time.Duration) (net.Conn, error) {
		return nil, nil
	}
}

func TestStreamLayer_UnspecifiedIP(t *testing.T) {
	m := &mockClusterHook{
		address: &cluster.NetAddr{
			Host: "0.0.0.0:8200",
		},
	}

	raftTLSKey, err := GenerateTLSKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	raftTLS := &TLSKeyring{
		Keys:        []*TLSKey{raftTLSKey},
		ActiveKeyID: raftTLSKey.ID,
	}

	layer, err := NewRaftLayer(nil, raftTLS, m)
	if err == nil {
		t.Fatal("expected error")
	}

	if err.Error() != "cannot use unspecified IP with raft storage: 0.0.0.0:8200" {
		t.Fatalf("unexpected error: %s", err.Error())
	}

	if layer != nil {
		t.Fatal("expected nil layer")
	}

	m.address.(*cluster.NetAddr).Host = "10.0.0.1:8200"

	layer, err = NewRaftLayer(nil, raftTLS, m)
	if err != nil {
		t.Fatal(err)
	}

	if layer == nil {
		t.Fatal("nil layer")
	}
}

// TestStreamLayer_SetTLSKeyringSameTerm checks that a keyring with the same
// term but different keys replaces the current one. two clusters that were
// initialised separately both start at term 0, so after a snapshot restore
// from one into the other the keys change while the term does not.
func TestStreamLayer_SetTLSKeyringSameTerm(t *testing.T) {
	m := &mockClusterHook{
		address: &cluster.NetAddr{
			Host: "10.0.0.1:8200",
		},
	}

	keyA, err := GenerateTLSKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	keyB, err := GenerateTLSKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	layer, err := NewRaftLayer(nil, &TLSKeyring{
		Keys:        []*TLSKey{keyA},
		ActiveKeyID: keyA.ID,
	}, m)
	if err != nil {
		t.Fatal(err)
	}

	restored := &TLSKeyring{
		Keys:        []*TLSKey{keyB},
		ActiveKeyID: keyB.ID,
	}
	if err := layer.setTLSKeyring(restored); err != nil {
		t.Fatal(err)
	}

	if got := layer.ServerName(); got != keyB.ID {
		t.Fatalf("server name: expected %q, got %q", keyB.ID, got)
	}

	cert, err := layer.ServerLookup(context.Background(), &tls.ClientHelloInfo{ServerName: keyB.ID})
	if err != nil {
		t.Fatal(err)
	}
	if cert == nil {
		t.Fatal("expected the restored key to be served for its own server name")
	}

	// an equal keyring in a different object is still a no-op
	same := &TLSKeyring{Keys: restored.Keys, ActiveKeyID: restored.ActiveKeyID}
	if err := layer.setTLSKeyring(same); err != nil {
		t.Fatal(err)
	}
	if layer.keyring != restored {
		t.Fatal("expected an equal keyring to be a no-op")
	}

	// the same keys with a different active key are not
	twoKeys := &TLSKeyring{Keys: []*TLSKey{keyA, keyB}, ActiveKeyID: keyA.ID}
	if err := layer.setTLSKeyring(twoKeys); err != nil {
		t.Fatal(err)
	}
	if got := layer.ServerName(); got != keyA.ID {
		t.Fatalf("server name: expected %q, got %q", keyA.ID, got)
	}
	switched := &TLSKeyring{Keys: []*TLSKey{keyA, keyB}, ActiveKeyID: keyB.ID}
	if err := layer.setTLSKeyring(switched); err != nil {
		t.Fatal(err)
	}
	if got := layer.ServerName(); got != keyB.ID {
		t.Fatalf("active key change was ignored: expected %q, got %q", keyB.ID, got)
	}
}
