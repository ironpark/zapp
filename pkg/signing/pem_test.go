package signing

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
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

func ecKey(t *testing.T) *ecdsa.PrivateKey {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return key
}

// testP12 is a self-signed certificate and key as PKCS#12, in the modern
// encryption neither Apple's tools nor rcodesign read reliably.
func testP12(t *testing.T, password string) []byte {
	t.Helper()
	key := ecKey(t)
	p12, err := pkcs12.Modern2023.Encode(key, testCert(t, "Zapp Test Signing", key, nil, nil), nil, password)
	if err != nil {
		t.Fatal(err)
	}
	return p12
}

func writePEM(t *testing.T, parts ...string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "bundle.pem")
	if err := os.WriteFile(path, []byte(strings.Join(parts, "")), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// privateKey is what every standard library private key implements.
type privateKey interface {
	Equal(crypto.PrivateKey) bool
}

// Every unencrypted key encoding is read, and the certificate matching the key
// is chosen wherever it sits, with the rest as its chain.
func TestCertificateFromPEM(t *testing.T) {
	caKey := ecKey(t)
	ca := testCert(t, "Zapp Test CA", caKey, nil, nil)
	// RSA only for the PKCS#1 encoding, which has no other key type.
	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	ecdsaKey := ecKey(t)
	rsaLeaf := testCert(t, "Zapp RSA", rsaKey, ca, caKey)
	ecLeaf := testCert(t, "Zapp EC", ecdsaKey, ca, caKey)
	ecDER, err := x509.MarshalECPrivateKey(ecdsaKey)
	if err != nil {
		t.Fatal(err)
	}

	for name, tc := range map[string]struct {
		bundle []string
		key    privateKey
		leaf   *x509.Certificate
		chain  []*x509.Certificate
	}{
		"pkcs8 key first":          {[]string{pkcs8PEM(t, ecdsaKey), certPEM(ecLeaf), certPEM(ca)}, ecdsaKey, ecLeaf, []*x509.Certificate{ca}},
		"chain before the leaf":    {[]string{certPEM(ca), certPEM(ecLeaf), pkcs8PEM(t, ecdsaKey)}, ecdsaKey, ecLeaf, []*x509.Certificate{ca}},
		"pkcs1 rsa key":            {[]string{certPEM(rsaLeaf), pemBlock("RSA PRIVATE KEY", x509.MarshalPKCS1PrivateKey(rsaKey)), certPEM(ca)}, rsaKey, rsaLeaf, []*x509.Certificate{ca}},
		"sec1 ec key and no chain": {[]string{"comment lines are ignored\n", pemBlock("EC PRIVATE KEY", ecDER), certPEM(ecLeaf)}, ecdsaKey, ecLeaf, nil},
	} {
		key, leaf, chain, err := Credentials{PEMFile: writePEM(t, tc.bundle...)}.certificate()
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if !tc.key.Equal(key) || !leaf.Equal(tc.leaf) {
			t.Fatalf("%s: wrong identity %s", name, leaf.Subject.CommonName)
		}
		if len(chain) != len(tc.chain) || (len(chain) == 1 && !chain[0].Equal(tc.chain[0])) {
			t.Fatalf("%s: chain = %d certificates", name, len(chain))
		}
	}

	for name, tc := range map[string]struct {
		bundle []string
		want   string
	}{
		"certificate only": {[]string{certPEM(ecLeaf)}, "no private key"},
		"key only":         {[]string{pkcs8PEM(t, ecdsaKey)}, "no certificate found"},
		"mismatched":       {[]string{pkcs8PEM(t, caKey), certPEM(ecLeaf)}, "no certificate matches"},
		"two keys":         {[]string{pkcs8PEM(t, ecdsaKey), pkcs8PEM(t, caKey), certPEM(ecLeaf)}, "more than one private key"},
		"encrypted key":    {[]string{pemBlock("ENCRYPTED PRIVATE KEY", []byte("x")), certPEM(ecLeaf)}, "encrypted"},
		"corrupt key":      {[]string{pemBlock("PRIVATE KEY", []byte("x")), certPEM(ecLeaf)}, "invalid private key"},
	} {
		if _, _, _, err := (Credentials{PEMFile: writePEM(t, tc.bundle...)}).certificate(); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: err = %v, want %q", name, err, tc.want)
		}
	}
	if _, _, _, err := (Credentials{PEMFile: filepath.Join(t.TempDir(), "missing.pem")}).certificate(); err == nil {
		t.Error("missing file accepted")
	}
}

// A PKCS#12 bundle in either encryption yields the same key, certificate and
// chain, and a wrong password says so.
func TestCertificateFromP12(t *testing.T) {
	caKey, key := ecKey(t), ecKey(t)
	ca := testCert(t, "Zapp Test CA", caKey, nil, nil)
	leaf := testCert(t, "Zapp EC", key, ca, caKey)
	for name, enc := range map[string]*pkcs12.Encoder{"modern": pkcs12.Modern2023, "legacy": pkcs12.LegacyDES} {
		p12, err := enc.Encode(key, leaf, []*x509.Certificate{ca}, "secret")
		if err != nil {
			t.Fatal(err)
		}
		creds := Credentials{P12Base64: base64.StdEncoding.EncodeToString(p12), P12Password: "secret"}
		gotKey, gotLeaf, chain, err := creds.certificate()
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if !key.Equal(gotKey) || !gotLeaf.Equal(leaf) || len(chain) != 1 || !chain[0].Equal(ca) {
			t.Fatalf("%s: certificate does not match the PKCS#12", name)
		}
		creds.P12Password = "wrong"
		if _, _, _, err := creds.certificate(); err == nil || !strings.Contains(err.Error(), "incorrect PKCS#12 password") {
			t.Fatalf("%s: wrong password: %v", name, err)
		}
	}
	if _, _, _, err := (Credentials{P12Base64: base64.StdEncoding.EncodeToString([]byte("not a bundle"))}).certificate(); err == nil {
		t.Fatal("accepted garbage")
	}
}

// Each backend's repackaging reads back as the same certificate.
func TestRepackaging(t *testing.T) {
	caKey, key := ecKey(t), ecKey(t)
	ca := testCert(t, "Zapp Test CA", caKey, nil, nil)
	leaf := testCert(t, "Zapp EC", key, ca, caKey)
	check := func(format string, gotKey crypto.PrivateKey, gotLeaf *x509.Certificate, chain []*x509.Certificate) {
		t.Helper()
		if !key.Equal(gotKey) || !gotLeaf.Equal(leaf) || len(chain) != 1 || !chain[0].Equal(ca) {
			t.Fatalf("%s does not match the original", format)
		}
	}

	p12, password, err := p12Bundle(key, leaf, []*x509.Certificate{ca})
	if err != nil {
		t.Fatal(err)
	}
	if len(password) < 26 {
		t.Fatalf("weak password %q", password)
	}
	gotKey, gotLeaf, chain, err := pkcs12.DecodeChain(p12, password)
	if err != nil {
		t.Fatal(err)
	}
	check("PKCS#12", gotKey, gotLeaf, chain)

	bundle, err := pemBundle(key, leaf, []*x509.Certificate{ca})
	if err != nil {
		t.Fatal(err)
	}
	gotKey, gotLeaf, chain, err = parsePEMBundle(bundle)
	if err != nil {
		t.Fatal(err)
	}
	check("PEM", gotKey, gotLeaf, chain)
}
