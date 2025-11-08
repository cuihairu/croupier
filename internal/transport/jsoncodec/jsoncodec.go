package jsoncodec

import (
	"encoding/json"

	"google.golang.org/grpc/encoding"
)

const Name = "json"

type codec struct{}

func (codec) Name() string                               { return Name }
func (codec) Marshal(v interface{}) ([]byte, error)      { return json.Marshal(v) }
func (codec) Unmarshal(data []byte, v interface{}) error { return json.Unmarshal(data, v) }

func init() {
	encoding.RegisterCodec(codec{})
}
