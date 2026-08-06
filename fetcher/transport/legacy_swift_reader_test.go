package transport_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/imgproxy/imgproxy/v4/fetcher/transport"
	"github.com/imgproxy/imgproxy/v4/storage"
)

// legacyCallRecordingStorage is a mock storage.Reader that records every key it was
// asked for and returns a canned response per key.
type legacyCallRecordingStorage struct {
	responses map[string]*storage.ObjectReader
	errs      map[string]error
	calls     []string
}

func (m *legacyCallRecordingStorage) GetObject(
	ctx context.Context, reqHeader http.Header, bucket, key, query string,
) (*storage.ObjectReader, error) {
	m.calls = append(m.calls, key)

	if err, ok := m.errs[key]; ok {
		return nil, err
	}

	if resp, ok := m.responses[key]; ok {
		return resp, nil
	}

	return storage.NewObjectNotFound("not found"), nil
}

func legacyReaderOK() *storage.ObjectReader {
	return storage.NewObjectOK(make(http.Header), io.NopCloser(strings.NewReader("data")))
}

type LegacyPercentDecodeReaderTestSuite struct {
	suite.Suite
}

func (s *LegacyPercentDecodeReaderTestSuite) TestLegacyKeyHits() {
	mock := &legacyCallRecordingStorage{
		responses: map[string]*storage.ObjectReader{
			"M 35437 15.jpg": legacyReaderOK(), // decoded (legacy) form
		},
	}

	r := transport.NewLegacyPercentDecodeReader(mock)
	obj, err := r.GetObject(context.Background(), nil, "bucket", "M%2035437%2015.jpg", "")
	s.Require().NoError(err)
	s.Equal(http.StatusOK, obj.Status)
	s.Equal([]string{"M 35437 15.jpg"}, mock.calls)
}

func (s *LegacyPercentDecodeReaderTestSuite) TestLegacyKey404FallsBackToLiteralKeyWhichHits() {
	mock := &legacyCallRecordingStorage{
		responses: map[string]*storage.ObjectReader{
			// legacy-decoded key "M%2035437%2015.jpg" isn't present -> default 404 -> falls back to this
			"M%252035437%252015.jpg": legacyReaderOK(), // literal (current v4) form
		},
	}

	r := transport.NewLegacyPercentDecodeReader(mock)
	obj, err := r.GetObject(context.Background(), nil, "bucket", "M%252035437%252015.jpg", "")
	s.Require().NoError(err)
	s.Equal(http.StatusOK, obj.Status)
	s.Equal([]string{"M%2035437%2015.jpg", "M%252035437%252015.jpg"}, mock.calls)
}

func (s *LegacyPercentDecodeReaderTestSuite) TestBothKeys404() {
	mock := &legacyCallRecordingStorage{responses: map[string]*storage.ObjectReader{}}

	r := transport.NewLegacyPercentDecodeReader(mock)
	obj, err := r.GetObject(context.Background(), nil, "bucket", "nonexistent%20file.jpg", "")
	s.Require().NoError(err)
	s.Equal(http.StatusNotFound, obj.Status)
	s.Equal([]string{"nonexistent file.jpg", "nonexistent%20file.jpg"}, mock.calls)
}

func (s *LegacyPercentDecodeReaderTestSuite) TestNoOpDecodeMakesOnlyOneCall() {
	mock := &legacyCallRecordingStorage{
		responses: map[string]*storage.ObjectReader{
			"plain-key.jpg": legacyReaderOK(),
		},
	}

	r := transport.NewLegacyPercentDecodeReader(mock)
	obj, err := r.GetObject(context.Background(), nil, "bucket", "plain-key.jpg", "")
	s.Require().NoError(err)
	s.Equal(http.StatusOK, obj.Status)
	s.Equal([]string{"plain-key.jpg"}, mock.calls)
}

func (s *LegacyPercentDecodeReaderTestSuite) TestMalformedPercentEncodingMakesOnlyOneCall() {
	mock := &legacyCallRecordingStorage{
		responses: map[string]*storage.ObjectReader{
			"trailing%.jpg": legacyReaderOK(),
		},
	}

	r := transport.NewLegacyPercentDecodeReader(mock)
	obj, err := r.GetObject(context.Background(), nil, "bucket", "trailing%.jpg", "")
	s.Require().NoError(err)
	s.Equal(http.StatusOK, obj.Status)
	s.Equal([]string{"trailing%.jpg"}, mock.calls)
}

func (s *LegacyPercentDecodeReaderTestSuite) TestLegacyKeyTransportErrorSurfacesImmediately() {
	mock := &legacyCallRecordingStorage{
		errs: map[string]error{
			"M 35437 15.jpg": errors.New("boom"),
		},
	}

	r := transport.NewLegacyPercentDecodeReader(mock)
	_, err := r.GetObject(context.Background(), nil, "bucket", "M%2035437%2015.jpg", "")
	s.Require().Error(err)
	s.Equal([]string{"M 35437 15.jpg"}, mock.calls)
}

func TestLegacyPercentDecodeReader(t *testing.T) {
	suite.Run(t, new(LegacyPercentDecodeReaderTestSuite))
}
