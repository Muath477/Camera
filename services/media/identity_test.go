package media

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"math/big"
	"net/http/httptest"
	"testing"
	"time"
)

func clientCert(t *testing.T, cn string, ou []string) string {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: cn, OrganizationalUnit: ou},
		NotBefore:    time.Now().Add(-time.Minute),
		NotAfter:     time.Now().Add(time.Hour),
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	return base64.StdEncoding.EncodeToString(der)
}

func TestDeviceIdentityFromGateway(t *testing.T) {
	const device, tenant = "22222222-2222-4222-8222-000000000001", "11111111-1111-4111-8111-111111111111"
	h := &Handler{TrustProxyIdentity: true}

	r := httptest.NewRequest("POST", "/device/v1/uploads", nil)
	r.Header.Set("X-Client-Cert", clientCert(t, device, []string{tenant}))
	d, tn, ok := h.DeviceIdentity(r)
	if !ok || d != device || tn != tenant {
		t.Fatalf("got %q %q %v", d, tn, ok)
	}

	cases := map[string]string{
		"missing tenant": clientCert(t, device, nil),
		"garbage":        "not base64",
		"empty":          "",
	}
	for name, v := range cases {
		r := httptest.NewRequest("POST", "/device/v1/uploads", nil)
		r.Header.Set("X-Client-Cert", v)
		if _, _, ok := h.DeviceIdentity(r); ok {
			t.Errorf("%s: identity accepted", name)
		}
	}

	// Without the gateway, headers mean nothing.
	plain := &Handler{}
	r = httptest.NewRequest("POST", "/device/v1/uploads", nil)
	r.Header.Set("X-Client-Cert", clientCert(t, device, []string{tenant}))
	if _, _, ok := plain.DeviceIdentity(r); ok {
		t.Error("header trusted without the gateway flag")
	}
}
