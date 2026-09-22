package compress

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"io"
	"regexp"
)

// Canonical base64, like the Node regex.
var base64Only = regexp.MustCompile(`^([A-Za-z0-9+/]{4})*([A-Za-z0-9+/]{4}|[A-Za-z0-9+/]{3}=|[A-Za-z0-9+/]{2}==)$`)

func IsBase64(s string) bool { return base64Only.MatchString(s) }

func DecodeGzipBase64(s string) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return "", err
	}
	zr, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		return "", err
	}
	defer func() { _ = zr.Close() }()
	out, err := io.ReadAll(zr)
	if err != nil {
		return "", err
	}
	return string(out), nil
}
