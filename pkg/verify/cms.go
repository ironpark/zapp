package verify

import (
	"crypto/x509"
	"encoding/asn1"
	"errors"
	"fmt"
	"time"
)

// signer is who made a CMS signature, and when a timestamp authority says it
// was made.
type signer struct {
	// Leaf is the signing certificate; Chain holds the rest.
	Leaf  *x509.Certificate
	Chain []*x509.Certificate
	// Timestamp is the zero time without a timestamp token.
	Timestamp time.Time
}

var (
	oidSignedData     = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 7, 2}
	oidTimestampToken = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 9, 16, 2, 14}
	oidTSTInfo        = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 9, 16, 1, 4}
)

type contentInfo struct {
	ContentType asn1.ObjectIdentifier
	Content     asn1.RawValue `asn1:"explicit,tag:0"`
}

type signedData struct {
	Version          int
	DigestAlgorithms asn1.RawValue
	EncapContentInfo encapContentInfo
	Certificates     asn1.RawValue `asn1:"optional,tag:0"`
	CRLs             asn1.RawValue `asn1:"optional,tag:1"`
	SignerInfos      asn1.RawValue
}

type encapContentInfo struct {
	ContentType asn1.ObjectIdentifier
	Content     asn1.RawValue `asn1:"optional,explicit,tag:0"`
}

type issuerSerial struct {
	Issuer asn1.RawValue
	Serial asn1.RawValue
}

type signerInfo struct {
	Version            int
	SID                asn1.RawValue
	DigestAlgorithm    asn1.RawValue
	SignedAttrs        asn1.RawValue `asn1:"optional,tag:0"`
	SignatureAlgorithm asn1.RawValue
	Signature          []byte
	UnsignedAttrs      asn1.RawValue `asn1:"optional,tag:1"`
}

type attribute struct {
	Type   asn1.ObjectIdentifier
	Values asn1.RawValue
}

// parseSigner reads the signer of a CMS SignedData, as codesign and
// productsign make them.
func parseSigner(der []byte) (*signer, error) {
	sd, err := parseSignedData(der)
	if err != nil {
		return nil, err
	}
	certs, err := x509.ParseCertificates(sd.Certificates.Bytes)
	if err != nil {
		return nil, fmt.Errorf("signature certificates: %w", err)
	}
	var si signerInfo
	if _, err := asn1.Unmarshal(sd.SignerInfos.Bytes, &si); err != nil {
		return nil, fmt.Errorf("signer info: %w", err)
	}
	s := &signer{}
	var id issuerSerial
	if _, err := asn1.Unmarshal(si.SID.FullBytes, &id); err == nil {
		for _, c := range certs {
			if string(c.RawIssuer) == string(id.Issuer.FullBytes) && string(c.SerialNumber.Bytes()) == string(trimSerial(id.Serial.Bytes)) {
				s.Leaf = c
			}
		}
	}
	for _, c := range certs {
		if c != s.Leaf {
			s.Chain = append(s.Chain, c)
		}
	}
	if s.Leaf == nil {
		return nil, errors.New("the signature does not include its signing certificate")
	}
	rest := si.UnsignedAttrs.Bytes
	for len(rest) > 0 {
		var a attribute
		if rest, err = asn1.Unmarshal(rest, &a); err != nil {
			return nil, fmt.Errorf("unsigned attributes: %w", err)
		}
		if a.Type.Equal(oidTimestampToken) {
			s.Timestamp, _ = timestampTime(a.Values.Bytes)
			if s.Timestamp.IsZero() {
				s.Timestamp = time.Unix(0, 0) // a token zapp cannot read is still one
			}
		}
	}
	return s, nil
}

func parseSignedData(ber []byte) (*signedData, error) {
	der, err := berToDER(ber)
	if err != nil {
		return nil, fmt.Errorf("signature is not CMS: %w", err)
	}
	var ci contentInfo
	if _, err := asn1.Unmarshal(der, &ci); err != nil {
		return nil, fmt.Errorf("signature is not CMS: %w", err)
	}
	if !ci.ContentType.Equal(oidSignedData) {
		return nil, errors.New("signature is not CMS signed data")
	}
	var sd signedData
	if _, err := asn1.Unmarshal(ci.Content.Bytes, &sd); err != nil {
		return nil, fmt.Errorf("signed data: %w", err)
	}
	return &sd, nil
}

// timestampTime reads genTime from an RFC 3161 timestamp token, itself CMS
// signed data wrapping a TSTInfo.
func timestampTime(token []byte) (time.Time, error) {
	sd, err := parseSignedData(token)
	if err != nil {
		return time.Time{}, err
	}
	if !sd.EncapContentInfo.ContentType.Equal(oidTSTInfo) {
		return time.Time{}, errors.New("timestamp token holds no TSTInfo")
	}
	var octets []byte
	if _, err := asn1.Unmarshal(sd.EncapContentInfo.Content.Bytes, &octets); err != nil {
		return time.Time{}, err
	}
	var info struct {
		Version        int
		Policy         asn1.ObjectIdentifier
		MessageImprint asn1.RawValue
		Serial         asn1.RawValue
		GenTime        time.Time `asn1:"generalized"`
	}
	_, err = asn1.Unmarshal(octets, &info)
	return info.GenTime, err
}

// trimSerial drops the sign byte DER puts before a serial with its high bit
// set, which big.Int.Bytes leaves out.
func trimSerial(b []byte) []byte {
	for len(b) > 1 && b[0] == 0 {
		b = b[1:]
	}
	return b
}
