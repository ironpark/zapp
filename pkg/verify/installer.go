package verify

import (
	"bytes"
	"compress/zlib"
	"crypto/x509"
	"encoding/base64"
	"encoding/binary"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"context"

	"github.com/ironpark/zapp/pkg/macho"
)

// DMG verifies a disk image's own signature. The app inside is not opened:
// verify it before it is packaged, as a build does.
func DMG(ctx context.Context, path string) Report {
	r := Report{Path: path}
	sig, err := dmgSignature(path)
	switch {
	case err != nil:
		r.add("signature", Fail, "%v", err)
		return r
	case sig == nil || !sig.Signed:
		r.add("signature", Fail, "the disk image is not signed")
		return r
	}
	var s sigChecks
	s.signature(filepath.Base(path), sig, false, false)
	s.report(&r, "Developer ID Application", 1, false)
	if sig.Stapled {
		r.add("stapled", Pass, "the notarization ticket is attached")
	} else {
		r.add("stapled", Warn, "no notarization ticket attached; Gatekeeper has to reach Apple the first time it opens")
	}
	deepDMG(ctx, &r, path)
	return r
}

// dmgSignature reads the code signature a disk image's koly trailer points
// to, or nil when it has none.
func dmgSignature(path string) (*macho.Signature, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	const trailerSize = 512
	if info.Size() < trailerSize {
		return nil, errors.New("too small to be a disk image")
	}
	koly := make([]byte, trailerSize)
	if _, err := f.ReadAt(koly, info.Size()-trailerSize); err != nil {
		return nil, err
	}
	if string(koly[:4]) != "koly" {
		return nil, errors.New("not a UDIF disk image: no koly trailer")
	}
	off, size := binary.BigEndian.Uint64(koly[296:]), binary.BigEndian.Uint64(koly[304:])
	if off == 0 || size == 0 {
		return nil, nil
	}
	if off+size > uint64(info.Size()) || size > 16<<20 {
		return nil, errors.New("the disk image's code signature runs past its end")
	}
	blob := make([]byte, size)
	if _, err := f.ReadAt(blob, int64(off)); err != nil {
		return nil, err
	}
	return macho.ParseSignature(blob)
}

// PKG verifies an installer package's signature.
func PKG(ctx context.Context, path string) Report {
	r := Report{Path: path}
	sig, err := readXARSignature(path)
	if err != nil {
		r.add("signature", Fail, "%v", err)
		return r
	}
	var s sigChecks
	where := filepath.Base(path)
	switch {
	case sig.cms != nil:
		s.signer(where, sig.cms)
	case sig.leaf != nil:
		s.signer(where, &signer{Leaf: sig.leaf})
		s.noTimestamp = nil // an RSA signature has nowhere to keep one
	default:
		s.unsigned = append(s.unsigned, where)
	}
	s.report(&r, "Developer ID Installer", 1, false)
	if sig.stapled {
		r.add("stapled", Pass, "the notarization ticket is attached")
	} else {
		r.add("stapled", Warn, "no notarization ticket attached; the installer has to reach Apple to check it")
	}
	deepPKG(ctx, &r, path)
	return r
}

type xarSignature struct {
	cms     *signer
	leaf    *x509.Certificate
	stapled bool
}

type xarSig struct {
	Offset int64    `xml:"offset"`
	Size   int64    `xml:"size"`
	Certs  []string `xml:"KeyInfo>X509Data>X509Certificate"`
}

// readXARSignature reads a flat package's signatures from its table of
// contents: the CMS one, which carries the timestamp, and the RSA one, whose
// certificates are listed in the table itself.
func readXARSignature(path string) (*xarSignature, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(data) < 28 || string(data[:4]) != "xar!" {
		return nil, errors.New("not a flat installer package: no xar header")
	}
	headerSize := int64(binary.BigEndian.Uint16(data[4:]))
	tocSize := int64(binary.BigEndian.Uint64(data[8:]))
	if headerSize+tocSize > int64(len(data)) {
		return nil, errors.New("the package's table of contents runs past its end")
	}
	zr, err := zlib.NewReader(bytes.NewReader(data[headerSize : headerSize+tocSize]))
	if err != nil {
		return nil, fmt.Errorf("the package's table of contents: %w", err)
	}
	tocXML, err := io.ReadAll(io.LimitReader(zr, 64<<20))
	if err != nil {
		return nil, fmt.Errorf("the package's table of contents: %w", err)
	}
	var toc struct {
		RSA []xarSig `xml:"toc>signature"`
		CMS []xarSig `xml:"toc>x-signature"`
	}
	if err := xml.Unmarshal(tocXML, &toc); err != nil {
		return nil, fmt.Errorf("the package's table of contents: %w", err)
	}
	heap := data[headerSize+tocSize:]
	sig := &xarSignature{stapled: len(data) >= 16 && string(data[len(data)-16:len(data)-12]) == "t8lr"}
	if sig.stapled {
		// The ticket and its trailers follow the heap.
		heap = heap[:len(heap)-16]
	}
	for _, x := range toc.CMS {
		if x.Offset >= 0 && x.Size > 0 && x.Offset+x.Size <= int64(len(heap)) {
			if sig.cms, err = parseSigner(heap[x.Offset : x.Offset+x.Size]); err != nil {
				return nil, fmt.Errorf("the package's CMS signature: %w", err)
			}
		}
	}
	for _, x := range toc.RSA {
		if len(x.Certs) > 0 {
			der, err := base64.StdEncoding.DecodeString(strings.Join(strings.Fields(x.Certs[0]), ""))
			if err != nil {
				return nil, fmt.Errorf("the package's certificate: %w", err)
			}
			if sig.leaf, err = x509.ParseCertificate(der); err != nil {
				return nil, fmt.Errorf("the package's certificate: %w", err)
			}
		}
	}
	return sig, nil
}
