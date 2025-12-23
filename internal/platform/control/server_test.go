package control

import (
	"bytes"
	"compress/gzip"
	"context"
	"testing"

	"github.com/cuihairu/croupier/pkg/pb/croupier/server/v1"
	"github.com/stretchr/testify/require"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func gzipBytes(t *testing.T, b []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	_, err := zw.Write(b)
	require.NoError(t, err)
	require.NoError(t, zw.Close())
	return buf.Bytes()
}

func findDetail[T any](details []interface{}) (T, bool) {
	var zero T
	for _, d := range details {
		if v, ok := d.(T); ok {
			return v, true
		}
	}
	return zero, false
}

func TestRegisterCapabilities_InvalidManifestJSON_ReturnsInvalidArgument(t *testing.T) {
	s := NewServer(nil)
	_, err := s.RegisterCapabilities(context.Background(), &serverv1.RegisterCapabilitiesRequest{
		Provider:       &serverv1.ProviderMeta{Id: "p", Version: "1"},
		ManifestJsonGz: gzipBytes(t, []byte("{")),
	})
	require.Error(t, err)

	st, ok := status.FromError(err)
	require.True(t, ok)
	require.Equal(t, codes.InvalidArgument, st.Code())

	details := st.Details()
	_, ok = findDetail[*errdetails.BadRequest](details)
	require.True(t, ok)
	ei, ok := findDetail[*errdetails.ErrorInfo](details)
	require.True(t, ok)
	require.Equal(t, "PROVIDER_MANIFEST_INVALID_JSON", ei.GetReason())
}

func TestRegisterCapabilities_SchemaViolation_ReturnsFieldViolations(t *testing.T) {
	s := NewServer(nil)
	_, err := s.RegisterCapabilities(context.Background(), &serverv1.RegisterCapabilitiesRequest{
		Provider:       &serverv1.ProviderMeta{Id: "p", Version: "1"},
		ManifestJsonGz: gzipBytes(t, []byte(`{"provider":{"id":"p"}}`)),
	})
	require.Error(t, err)

	st, ok := status.FromError(err)
	require.True(t, ok)
	require.Equal(t, codes.InvalidArgument, st.Code())

	br, ok := findDetail[*errdetails.BadRequest](st.Details())
	require.True(t, ok)
	require.NotEmpty(t, br.GetFieldViolations())
}

func TestRegisterCapabilities_ValidManifest_Succeeds(t *testing.T) {
	s := NewServer(nil)
	_, err := s.RegisterCapabilities(context.Background(), &serverv1.RegisterCapabilitiesRequest{
		Provider:       &serverv1.ProviderMeta{Id: "p", Version: "1"},
		ManifestJsonGz: gzipBytes(t, []byte(`{"provider":{"id":"p","version":"1"}}`)),
	})
	require.NoError(t, err)
}
