package dbtype

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// json.RawMessage.UnmarshalJSON always returns a nil error for a non-empty
// receiver (raw bytes pass through; no validation happens at parse time), so
// the error branch of UnmarshalJSON is only reachable when a failure is
// injected through the seam. Verifies that the error is propagated verbatim
// and the receiver keeps its original value instead of being overwritten.
func TestJSONUnmarshalJSON_RawMessageErrorSeam(t *testing.T) {
	orig := unmarshalRawMessage
	unmarshalRawMessage = func(*json.RawMessage, []byte) error {
		return errors.New("raw message explode")
	}
	t.Cleanup(func() { unmarshalRawMessage = orig })

	var j = JSON(`{"keep":1}`)
	err := j.UnmarshalJSON([]byte(`{"x":1}`))
	require.EqualError(t, err, "raw message explode")
	assert.Equal(t, `{"keep":1}`, j.String())
}
