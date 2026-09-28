package signing

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"software.sslmate.com/src/go-pkcs12"
)

// testCert makes a code signing certificate for key, signed by parent (or
// self-signed when parent is nil).
func testCert(t *testing.T, name string, key crypto.Signer, parent *x509.Certificate, parentKey crypto.Signer) *x509.Certificate {
	t.Helper()
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(time.Now().UnixNano()),
		Subject:               pkix.Name{CommonName: name},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageCodeSigning},
		BasicConstraintsValid: true,
		IsCA:                  parent == nil,
	}
	if parent == nil {
		parent, parentKey = tmpl, key
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, parent, key.Public(), parentKey)
	if err != nil {
		t.Fatal(err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	return cert
}

func pemBlock(typ string, der []byte) string {
	return string(pem.EncodeToMemory(&pem.Block{Type: typ, Bytes: der}))
}

func certPEM(c *x509.Certificate) string { return pemBlock("CERTIFICATE", c.Raw) }

func pkcs8PEM(t *testing.T, key crypto.Signer) string {
	t.Helper()
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	return pemBlock("PRIVATE KEY", der)
}

func writePEM(t *testing.T, parts ...string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "bundle.pem")
	if err := os.WriteFile(path, []byte(strings.Join(parts, "")), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// Every unencrypted key encoding is read, and the certificate matching the key
// is chosen as the identity wherever it sits, with the rest as its chain. The
// PKCS#12 decodes back to the same key and certificates.
func TestPEMToP12(t *testing.T) {
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	ca := testCert(t, "Zapp Test CA", caKey, nil, nil)
	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	ecKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	rsaLeaf := testCert(t, "Zapp RSA", rsaKey, ca, caKey)
	ecLeaf := testCert(t, "Zapp EC", ecKey, ca, caKey)
	ecDER, err := x509.MarshalECPrivateKey(ecKey)
	if err != nil {
		t.Fatal(err)
	}

	for name, tc := range map[string]struct {
		bundle []string
		key    crypto.Signer
		leaf   *x509.Certificate
	}{
		"pkcs8 key first":          {[]string{pkcs8PEM(t, rsaKey), certPEM(rsaLeaf), certPEM(ca)}, rsaKey, rsaLeaf},
		"chain before the leaf":    {[]string{certPEM(ca), certPEM(rsaLeaf), pkcs8PEM(t, rsaKey)}, rsaKey, rsaLeaf},
		"pkcs1 rsa key":            {[]string{certPEM(rsaLeaf), pemBlock("RSA PRIVATE KEY", x509.MarshalPKCS1PrivateKey(rsaKey)), certPEM(ca)}, rsaKey, rsaLeaf},
		"sec1 ec key and no chain": {[]string{"comment lines are ignored\n", pemBlock("EC PRIVATE KEY", ecDER), certPEM(ecLeaf)}, ecKey, ecLeaf},
	} {
		p12, password, err := pemToP12(writePEM(t, tc.bundle...))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if len(password) < 32 {
			t.Fatalf("%s: weak password %q", name, password)
		}
		key, leaf, chain, err := pkcs12.DecodeChain(p12, password)
		if err != nil {
			t.Fatalf("%s: decode: %v", name, err)
		}
		if !leaf.Equal(tc.leaf) || !tc.key.Public().(interface{ Equal(crypto.PublicKey) bool }).Equal(key.(crypto.Signer).Public()) {
			t.Fatalf("%s: wrong identity %s", name, leaf.Subject.CommonName)
		}
		wantChain := 1
		if tc.leaf == ecLeaf {
			wantChain = 0
		}
		if len(chain) != wantChain || (wantChain == 1 && !chain[0].Equal(ca)) {
			t.Fatalf("%s: chain = %d certificates", name, len(chain))
		}
	}

	for name, tc := range map[string]struct {
		bundle []string
		want   string
	}{
		"certificate only": {[]string{certPEM(rsaLeaf)}, "no private key"},
		"key only":         {[]string{pkcs8PEM(t, rsaKey)}, "no certificate found"},
		"mismatched":       {[]string{pkcs8PEM(t, ecKey), certPEM(rsaLeaf)}, "no certificate matches"},
		"two keys":         {[]string{pkcs8PEM(t, rsaKey), pkcs8PEM(t, ecKey), certPEM(rsaLeaf)}, "more than one private key"},
		"encrypted key":    {[]string{pemBlock("ENCRYPTED PRIVATE KEY", []byte("x")), certPEM(rsaLeaf)}, "encrypted"},
		"corrupt key":      {[]string{pemBlock("PRIVATE KEY", []byte("x")), certPEM(rsaLeaf)}, "invalid private key"},
	} {
		if _, _, err := pemToP12(writePEM(t, tc.bundle...)); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: err = %v, want %q", name, err, tc.want)
		}
	}
	if _, _, err := pemToP12(filepath.Join(t.TempDir(), "missing.pem")); err == nil {
		t.Error("missing file accepted")
	}
}

// A PKCS#12 bundle in either encryption unpacks to a PEM bundle of the same
// key, certificate and chain, and a wrong password says so.
func TestP12ToPEM(t *testing.T) {
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	ca := testCert(t, "Zapp Test CA", caKey, nil, nil)
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	leaf := testCert(t, "Zapp RSA", key, ca, caKey)
	for name, enc := range map[string]*pkcs12.Encoder{"modern": pkcs12.Modern2023, "legacy": pkcs12.LegacyDES} {
		p12, err := enc.Encode(key, leaf, []*x509.Certificate{ca}, "secret")
		if err != nil {
			t.Fatal(err)
		}
		bundle, err := p12ToPEM(p12, "secret")
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		gotKey, gotLeaf, chain, err := parsePEMBundle(bundle)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if !gotLeaf.Equal(leaf) || len(chain) != 1 || !chain[0].Equal(ca) || !key.Equal(gotKey) {
			t.Fatalf("%s: bundle does not match the PKCS#12", name)
		}
		if _, err := p12ToPEM(p12, "wrong"); err == nil || !strings.Contains(err.Error(), "incorrect PKCS#12 password") {
			t.Fatalf("%s: wrong password: %v", name, err)
		}
	}
	if _, err := p12ToPEM([]byte("not a bundle"), ""); err == nil {
		t.Fatal("accepted garbage")
	}
}
