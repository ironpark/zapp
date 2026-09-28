package signing

import (
	"crypto"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"os"

	"software.sslmate.com/src/go-pkcs12"
)

// namesCertificate reports whether c supplies a signing certificate in any
// form.
func (c Credentials) namesCertificate() bool { return c.PEMFile != "" || c.namesP12() }

// certificate loads the signing certificate c names, from a PEM bundle or a
// PKCS#12 bundle in any encryption, as its private key, the certificate for
// that key, and the rest of the chain. Each backend then repackages it in the
// one form its tools read reliably, so every host reports a bad certificate,
// key or password the same way before any tool runs.
func (c Credentials) certificate() (crypto.PrivateKey, *x509.Certificate, []*x509.Certificate, error) {
	if c.PEMFile != "" {
		data, err := os.ReadFile(c.PEMFile)
		if err != nil {
			return nil, nil, nil, err
		}
		key, cert, chain, err := parsePEMBundle(data)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("%s: %w", c.PEMFile, err)
		}
		return key, cert, chain, nil
	}
	p12, err := c.p12()
	if err != nil {
		return nil, nil, nil, err
	}
	password, err := c.p12Password()
	if err != nil {
		return nil, nil, nil, err
	}
	key, cert, chain, err := pkcs12.DecodeChain(p12, password)
	if errors.Is(err, pkcs12.ErrIncorrectPassword) {
		return nil, nil, nil, errors.New("incorrect PKCS#12 password")
	}
	if err != nil {
		return nil, nil, nil, fmt.Errorf("could not read the PKCS#12 certificate: %w", err)
	}
	return key, cert, chain, nil
}

// p12Bundle packages a certificate as PKCS#12 under a random password, for
// Apple's tools. Legacy encryption is what every macOS release's
// `security import` reads; the bundle only ever exists in memory and in the
// private temp directory the import uses, under a password thrown away with it.
func p12Bundle(key crypto.PrivateKey, cert *x509.Certificate, chain []*x509.Certificate) ([]byte, string, error) {
	password := rand.Text()
	p12, err := pkcs12.LegacyDES.Encode(key, cert, chain, password)
	if err != nil {
		return nil, "", fmt.Errorf("could not package the certificate as PKCS#12: %w", err)
	}
	return p12, password, nil
}

// pemBundle writes a certificate as a PEM bundle of its private key,
// certificate and chain, for rcodesign. Its own PKCS#12 reader only
// understands the legacy 3DES/RC2 encryption and misreports anything newer,
// such as OpenSSL 3's default AES export, as a wrong password; it reads PEM in
// any case.
func pemBundle(key crypto.PrivateKey, cert *x509.Certificate, chain []*x509.Certificate) ([]byte, error) {
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return nil, fmt.Errorf("unsupported private key: %w", err)
	}
	out := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
	for _, c := range append([]*x509.Certificate{cert}, chain...) {
		out = append(out, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: c.Raw})...)
	}
	return out, nil
}

// parsePEMBundle splits a PEM bundle into its private key, the certificate
// for that key, wherever it sits in the file, and the remaining certificates.
func parsePEMBundle(data []byte) (crypto.PrivateKey, *x509.Certificate, []*x509.Certificate, error) {
	var key crypto.Signer
	var certs []*x509.Certificate
	for {
		var block *pem.Block
		block, data = pem.Decode(data)
		if block == nil {
			break
		}
		switch block.Type {
		case "CERTIFICATE":
			cert, err := x509.ParseCertificate(block.Bytes)
			if err != nil {
				return nil, nil, nil, fmt.Errorf("invalid certificate: %w", err)
			}
			certs = append(certs, cert)
		case "PRIVATE KEY", "RSA PRIVATE KEY", "EC PRIVATE KEY":
			if key != nil {
				return nil, nil, nil, errors.New("more than one private key")
			}
			k, err := parsePrivateKey(block)
			if err != nil {
				return nil, nil, nil, err
			}
			key = k
		case "ENCRYPTED PRIVATE KEY":
			return nil, nil, nil, errors.New("the private key is encrypted; decrypt it (openssl pkcs8 -in key.pem -out plain.pem) or use a PKCS#12 certificate")
		}
	}
	if key == nil {
		return nil, nil, nil, errors.New("no private key found")
	}
	pub, ok := key.Public().(interface{ Equal(crypto.PublicKey) bool })
	if !ok {
		return nil, nil, nil, errors.New("unsupported private key type")
	}
	for i, cert := range certs {
		if pub.Equal(cert.PublicKey) {
			chain := append(certs[:i:i], certs[i+1:]...)
			return key, cert, chain, nil
		}
	}
	if len(certs) == 0 {
		return nil, nil, nil, errors.New("no certificate found")
	}
	return nil, nil, nil, errors.New("no certificate matches the private key")
}

func parsePrivateKey(block *pem.Block) (crypto.Signer, error) {
	var k any
	var err error
	switch block.Type {
	case "RSA PRIVATE KEY":
		k, err = x509.ParsePKCS1PrivateKey(block.Bytes)
	case "EC PRIVATE KEY":
		k, err = x509.ParseECPrivateKey(block.Bytes)
	default:
		k, err = x509.ParsePKCS8PrivateKey(block.Bytes)
	}
	if err != nil {
		return nil, fmt.Errorf("invalid private key: %w", err)
	}
	signer, ok := k.(crypto.Signer)
	if !ok {
		return nil, errors.New("unsupported private key type")
	}
	return signer, nil
}
