package httpserver

import (
	"net/http"
	"time"
	"unsafe"

	ginrender "github.com/gin-gonic/gin/render"
	jsoniter "github.com/json-iterator/go"
	"github.com/modern-go/reflect2"
)

// jsonAPI is a json-iterator instance compatible with encoding/json,
// but with a custom encoder for time.Time to emit RFC3339 without fractional seconds.
// timeRFC3339Encoder encodes time.Time values using RFC3339 without fractional seconds.
type timeRFC3339Encoder struct{}

func (e *timeRFC3339Encoder) IsEmpty(ptr unsafe.Pointer) bool {
	t := *((*time.Time)(ptr))
	return t.IsZero()
}
func (e *timeRFC3339Encoder) Encode(ptr unsafe.Pointer, stream *jsoniter.Stream) {
	t := *((*time.Time)(ptr))
	stream.WriteString(t.Format(time.RFC3339))
}

// timeExt registers encoder for time.Time on the per-API basis.
type timeExt struct{ jsoniter.DummyExtension }

func (e *timeExt) CreateEncoder(typ reflect2.Type) jsoniter.ValEncoder {
	tt := reflect2.TypeOfPtr((*time.Time)(nil)).Elem()
	if typ == tt {
		return &timeRFC3339Encoder{}
	}
	return nil
}

var jsonAPI = func() jsoniter.API {
	api := jsoniter.ConfigCompatibleWithStandardLibrary
	api.RegisterExtension(&timeExt{})
	return api
}()

// JSONRFC renders JSON using json-iterator with our global options.
type JSONRFC struct{ Data any }

func (r JSONRFC) Render(w http.ResponseWriter) error {
	r.WriteContentType(w)
	enc := jsonAPI.NewEncoder(w)
	return enc.Encode(r.Data)
}
func (r JSONRFC) WriteContentType(w http.ResponseWriter) {
	header := w.Header()
	if val := header["Content-Type"]; len(val) == 0 {
		header["Content-Type"] = []string{"application/json; charset=utf-8"}
	}
}

// JSON is the unified JSON responder; prefer this over c.JSON to ensure global settings apply.
func (s *Server) JSON(c Context, code int, v any) {
	// gin.Context satisfies this minimal interface; defined here to ease testing.
	type ginCtx interface {
		Status(int)
		Render(int, ginrender.Render)
	}
	if gc, ok := any(c).(ginCtx); ok {
		gc.Status(code)
		gc.Render(code, JSONRFC{Data: v})
		return
	}
}

// Context is a narrow interface of gin.Context used by Server.JSON (for testability).
type Context interface{}
