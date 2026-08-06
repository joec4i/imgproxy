package transport

import (
	"context"
	"net/http"
	"net/url"

	"github.com/imgproxy/imgproxy/v4/storage"
)

// legacyPercentDecodeReader wraps a storage.Reader and, before looking up an object,
// tries an extra percent-decode pass of the key first. This reproduces the pre-3.25
// behavior of building the Swift object key from Go's `net/url`-decoded path, so that
// source URLs built for that old behavior keep working after upgrading.
//
// It only falls back to the literal (current) key on a 404 response, never on a
// transport/auth error, so real errors aren't masked by a second lookup.
type legacyPercentDecodeReader struct {
	storage.Reader
}

// NewLegacyPercentDecodeReader wraps r with the legacy-percent-decode-first fallback
// behavior described by IMGPROXY_SWIFT_LEGACY_PERCENT_DECODE_FIRST.
func NewLegacyPercentDecodeReader(r storage.Reader) storage.Reader {
	return legacyPercentDecodeReader{r}
}

// GetObject implements storage.Reader.
func (r legacyPercentDecodeReader) GetObject(
	ctx context.Context, reqHeader http.Header, bucket, key, query string,
) (*storage.ObjectReader, error) {
	// url.PathUnescape (not url.QueryUnescape) matches Go's old net/url path-component
	// decode semantics exactly, so `+` is kept literal rather than turned into a space.
	legacyKey, err := url.PathUnescape(key)
	if err != nil || legacyKey == key {
		return r.Reader.GetObject(ctx, reqHeader, bucket, key, query)
	}

	obj, err := r.Reader.GetObject(ctx, reqHeader, bucket, legacyKey, query)
	if err != nil || obj.Status != http.StatusNotFound {
		return obj, err
	}

	return r.Reader.GetObject(ctx, reqHeader, bucket, key, query)
}
