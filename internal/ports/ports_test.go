package ports

import (
	"net"
	"strconv"
	"testing"
)

func TestFree_ReturnsUsablePort(t *testing.T) {
	p, err := Free()
	if err != nil {
		t.Fatalf("Free() error: %v", err)
	}
	if p <= 0 || p > 65535 {
		t.Fatalf("Free() = %d, out of range", p)
	}
	// The port must be bindable right after Free returns.
	l, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(p)))
	if err != nil {
		t.Fatalf("returned port %d not bindable: %v", p, err)
	}
	_ = l.Close()
}
