package signing

import (
	"crypto"
	"crypto/rand"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"os"

	"software.sslmate.com/src/go-pkcs12"
)

// pemToP12 reads a PEM bundle holding a private key and its certificate, plus
// any intermediate certificates, and repackages it as PKCS#12 under a random
// password, so Apple's tools can import it the way they import a supplied
// PKCS#12. The certificate is the one matching the key, wherever it sits in
// the file; the others travel along as its chain.
func pemToP12(path string) (p12 []byte, password string, err error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, "", err
	}
	key, cert, chain, err := parsePEMBundle(data)
	if err != nil {
		return nil, "", fmt.Errorf("%s: %w", path, err)
	}
	secret := make([]byte, 24)
	if _, err := rand.Read(secret); err != nil {
		return nil, "", err
	}
	password = hex.EncodeToString(secret)
	// Legacy encryption is what every macOS release's `security import` reads.
	// The bundle only ever exists in memory and in the private temp directory
	// the import uses, under a password that is thrown away with it.
	p12, err = pkcs12.LegacyDES.Encode(key, cert, chain, password)
	if err != nil {
		return nil, "", fmt.Errorf("could not package %s as PKCS#12: %w", path, err)
	}
	return p12, password, nil
}

// parsePEMBundle splits a PEM bundle into its private key, the certificate
// for that key, and the remaining certificates.
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

// p12ToPEM unpacks a PKCS#12 bundle into a PEM bundle of its private key,
// certificate and chain. rcodesign's PKCS#12 reader only understands the
// legacy 3DES/RC2 encryption and misreports anything newer, such as OpenSSL
// 3's default AES export, as a wrong password; it reads PEM in any case.
func p12ToPEM(p12 []byte, password string) ([]byte, error) {
	key, cert, chain, err := pkcs12.DecodeChain(p12, password)
	if errors.Is(err, pkcs12.ErrIncorrectPassword) {
		return nil, errors.New("incorrect PKCS#12 password")
	}
	if err != nil {
		return nil, fmt.Errorf("could not read the PKCS#12 certificate: %w", err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return nil, fmt.Errorf("unsupported private key in the PKCS#12 certificate: %w", err)
	}
	out := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
	for _, c := range append([]*x509.Certificate{cert}, chain...) {
		out = append(out, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: c.Raw})...)
	}
	return out, nil
}
