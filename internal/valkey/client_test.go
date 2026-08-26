package valkey

import (
	"net"
	"testing"
)

func TestGetSetRoundTrip(t *testing.T) {
	t.Parallel()
	ln := listenMem(t)
	c := &Client{Addr: ln.Addr().String()}

	got, err := c.Get(t.Context(), "missing")
	if err != nil || got != nil {
		t.Fatalf("missing %q %v", got, err)
	}
	if err := c.Set(t.Context(), "k", []byte(`{"ok":true}`)); err != nil {
		t.Fatal(err)
	}
	got, err = c.Get(t.Context(), "k")
	if err != nil || string(got) != `{"ok":true}` {
		t.Fatalf("get %q %v", got, err)
	}
}

func TestEmptyAddr(t *testing.T) {
	t.Parallel()
	if _, err := (&Client{}).Get(t.Context(), "k"); err == nil {
		t.Fatal("expected error")
	}
}

func listenMem(t *testing.T) net.Listener {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	go (&Memory{}).Listen(ln)
	return ln
}
