// Package tlsx manages optional mkcert-backed TLS for lane (https://*.localhost).
package tlsx

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/dheeraj-nalapat/lane/internal/paths"
)

// CertPath / KeyPath are the wildcard cert lane serves.
func CertPath() string { return filepath.Join(paths.TraefikCerts(), "cert.pem") }
func KeyPath() string  { return filepath.Join(paths.TraefikCerts(), "key.pem") }

// TLSConfigPath is the Traefik file-provider TLS config.
func TLSConfigPath() string { return filepath.Join(paths.TraefikDynamic(), "tls.yml") }

// Enabled reports whether TLS is on (the wildcard cert exists).
func Enabled() bool {
	_, err := os.Stat(CertPath())
	return err == nil
}

// MkcertInstalled reports whether the mkcert binary is on PATH.
func MkcertInstalled() bool {
	_, err := exec.LookPath("mkcert")
	return err == nil
}

// CAPresent reports whether a mkcert CA root exists (rootCA.pem under -CAROOT).
// Presence is a proxy for "the user has set up mkcert"; full trust-store
// installation still requires `mkcert -install`.
func CAPresent() bool {
	out, err := exec.Command("mkcert", "-CAROOT").Output()
	if err != nil {
		return false
	}
	caroot := strings.TrimSpace(string(out))
	if caroot == "" {
		return false
	}
	_, err = os.Stat(filepath.Join(caroot, "rootCA.pem"))
	return err == nil
}

// CertNames are the SANs lane requests for the wildcard cert.
func CertNames() []string {
	return []string{"*.localhost", "*.*.localhost", "localhost"}
}

func mkcertArgs(certPath, keyPath string) []string {
	return append([]string{"-cert-file", certPath, "-key-file", keyPath}, CertNames()...)
}

// Generate runs mkcert to write the wildcard cert/key (CA must already exist).
func Generate() error {
	if err := paths.Ensure(); err != nil {
		return err
	}
	cmd := exec.Command("mkcert", mkcertArgs(CertPath(), KeyPath())...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("mkcert generate failed: %v\n%s", err, out)
	}
	return nil
}

// RenderTLSConfig is the Traefik file-provider config naming the default cert.
func RenderTLSConfig() []byte {
	return []byte(`tls:
  stores:
    default:
      defaultCertificate:
        certFile: /certs/cert.pem
        keyFile: /certs/key.pem
`)
}

// WriteTLSConfig writes the TLS dynamic config.
func WriteTLSConfig() error {
	if err := paths.Ensure(); err != nil {
		return err
	}
	return os.WriteFile(TLSConfigPath(), RenderTLSConfig(), 0o644)
}

// Remove deletes the cert, key, and TLS config (disables TLS).
func Remove() error {
	for _, p := range []string{CertPath(), KeyPath(), TLSConfigPath()} {
		if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}
