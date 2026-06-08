// Package ports allocates free TCP ports on the host.
package ports

import "net"

// Free asks the kernel for an unused TCP port by binding to :0, then releases
// it. There is an inherent TOCTOU window, but it is sufficient for picking a
// Tilt UI port at startup.
func Free() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}
