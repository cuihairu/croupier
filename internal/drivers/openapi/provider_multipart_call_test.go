package openapi

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"strings"
	"testing"

	"github.com/cuihairu/croupier/internal/platform/provider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// SUI-013 acceptance: the executor transmits the file fields the upload
// contract describes — one file, a file array, and companion fields — and the
// upstream receives the real file bytes, not the URLs the web Upload widget
// produced.

const sui013AvatarData = "iVBORw0KGgo="
const sui013AvatarDataB = "R0lGODlhAQ=="

func dataURL(data string, media string) string {
	return "data:" + media + ";base64," + data
}

// readerForm parses a buildMultipartBody reader with the boundary the
// serializer announced in its Content-Type.
func readerForm(reader io.Reader, contentType string) (*multipart.Form, error) {
	raw, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	_, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		return nil, err
	}
	return multipart.NewReader(strings.NewReader(string(raw)), params["boundary"]).ReadForm(1 << 20)
}

// firstString returns the first value of a text form field.
func firstString(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func TestBuildMultipartBody_SingleFile(t *testing.T) {
	body, ct, err := buildMultipartBody(&RequestBodyMapping{
		Type:   "multipart",
		Fields: map[string]string{"avatar": "avatar"},
	}, map[string]interface{}{
		"avatar": map[string]interface{}{
			"filename": "avatar.png",
			"content":  dataURL(sui013AvatarData, "image/png"),
		},
	})
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(ct, "multipart/form-data; boundary="), "got %q", ct)

	form, err := readerForm(body, ct)
	require.NoError(t, err)
	require.Len(t, form.File["avatar"], 1)
	file := form.File["avatar"][0]
	assert.Equal(t, "avatar.png", file.Filename)
	fileReader, err := file.Open()
	require.NoError(t, err)
	content, err := io.ReadAll(fileReader)
	_ = fileReader.Close()
	require.NoError(t, err)
	assert.Equal(t, sui013AvatarData, base64.StdEncoding.EncodeToString(content))
}

func TestBuildMultipartBody_FileArray(t *testing.T) {
	body, ct, err := buildMultipartBody(&RequestBodyMapping{Type: "multipart"}, map[string]interface{}{
		"avatars": []interface{}{
			map[string]interface{}{"filename": "a.png", "content": dataURL(sui013AvatarData, "image/png")},
			map[string]interface{}{"filename": "b.gif", "content": dataURL(sui013AvatarDataB, "image/gif")},
		},
	})
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(ct, "multipart/form-data; boundary="), "got %q", ct)

	form, err := readerForm(body, ct)
	require.NoError(t, err)
	require.Len(t, form.File["avatars"], 2)
	assert.Equal(t, "a.png", form.File["avatars"][0].Filename)
	assert.Equal(t, "b.gif", form.File["avatars"][1].Filename)
	secondReader, err := form.File["avatars"][1].Open()
	require.NoError(t, err)
	second, err := io.ReadAll(secondReader)
	_ = secondReader.Close()
	require.NoError(t, err)
	assert.Equal(t, sui013AvatarDataB, base64.StdEncoding.EncodeToString(second))
}

func TestBuildMultipartBody_CompanionFieldsAndDerivedFilename(t *testing.T) {
	body, ct, err := buildMultipartBody(&RequestBodyMapping{
		Type:   "multipart",
		Fields: map[string]string{"playerId": "playerId", "avatar": "avatar", "note": "note"},
	}, map[string]interface{}{
		"playerId": "p-1",
		"avatar":   dataURL(sui013AvatarData, "image/png"), // bare data URL, no object
		"note":     "hello world",
	})
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(ct, "multipart/form-data; boundary="), "got %q", ct)

	form, err := readerForm(body, ct)
	require.NoError(t, err)
	assert.Equal(t, "p-1", firstString(form.Value["playerId"]))
	assert.Equal(t, "hello world", firstString(form.Value["note"]))
	require.Len(t, form.File["avatar"], 1)
	assert.Equal(t, "upload.png", form.File["avatar"][0].Filename,
		"a bare data URL derives its filename from the media type")
	avatarReader, err := form.File["avatar"][0].Open()
	require.NoError(t, err)
	content, err := io.ReadAll(avatarReader)
	_ = avatarReader.Close()
	require.NoError(t, err)
	assert.Equal(t, sui013AvatarData, base64.StdEncoding.EncodeToString(content))
}

func TestBuildMultipartBody_RawBase64Content(t *testing.T) {
	body, ct, err := buildMultipartBody(&RequestBodyMapping{Type: "multipart"}, map[string]interface{}{
		"payload": map[string]interface{}{
			"filename": "raw.bin",
			"content":  sui013AvatarData,
		},
	})
	require.NoError(t, err)
	form, err := readerForm(body, ct)
	require.NoError(t, err)
	require.Len(t, form.File["payload"], 1)
	payloadReader, err := form.File["payload"][0].Open()
	require.NoError(t, err)
	content, err := io.ReadAll(payloadReader)
	_ = payloadReader.Close()
	require.NoError(t, err)
	assert.Equal(t, sui013AvatarData, base64.StdEncoding.EncodeToString(content))
}

func TestBuildMultipartBody_SkipsNilAndRejectsBadPayload(t *testing.T) {
	body, ct, err := buildMultipartBody(&RequestBodyMapping{Type: "multipart"}, map[string]interface{}{
		"empty": nil,
		"note":  "kept",
	})
	require.NoError(t, err)
	form, err := readerForm(body, ct)
	require.NoError(t, err)
	assert.Empty(t, form.Value["empty"])
	assert.Equal(t, "kept", firstString(form.Value["note"]))

	_, _, err = buildMultipartBody(&RequestBodyMapping{Type: "multipart"}, map[string]interface{}{
		"bad": map[string]interface{}{"filename": "x", "content": "!!not-base64!!"},
	})
	assert.Error(t, err, "an undecodable payload must fail before the request leaves")

	_, _, err = buildMultipartBody(&RequestBodyMapping{Type: "multipart"}, map[string]interface{}{
		"bad": 12345,
	})
	assert.Error(t, err, "a non-upload scalar must fail before the request leaves")
}

func TestBuildMultipartBody_IsDeterministic(t *testing.T) {
	config := &RequestBodyMapping{
		Type:   "multipart",
		Fields: map[string]string{"a": "a", "b": "b"},
	}
	data := map[string]interface{}{
		"a": "one",
		"b": map[string]interface{}{"filename": "b.png", "content": dataURL(sui013AvatarData, "image/png")},
	}
	first, _, err := buildMultipartBody(config, data)
	require.NoError(t, err)
	second, _, err := buildMultipartBody(config, data)
	require.NoError(t, err)
	firstText, err := io.ReadAll(first)
	require.NoError(t, err)
	secondText, err := io.ReadAll(second)
	require.NoError(t, err)
	// The random boundary differs per call, so determinism is asserted on the
	// part payloads after the first blank line of each part.
	assert.Equal(t, partPayloads(string(firstText)), partPayloads(string(secondText)))
}

// partPayloads strips the boundary-dependent framing: everything up to the
// first blank line of each part is headers and the boundary, the payload and
// the disposition lines stay.
func partPayloads(body string) []string {
	var payloads []string
	for _, section := range strings.Split(body, "--") {
		if index := strings.Index(section, "\r\n\r\n"); index >= 0 {
			disposition := strings.SplitN(section, "\r\n", 3)
			payloads = append(payloads, strings.Join(disposition[1:2], "")+section[index+4:])
		}
	}
	return payloads
}

// The executor must actually ship the multipart body: the Content-Type with
// its boundary and the received file bytes are what the game side parses.
func TestCall_MultipartRequestBody_SendsFileParts(t *testing.T) {
	var gotContentType string
	var gotRaw []byte
	server := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		gotContentType = r.Header.Get("Content-Type")
		gotRaw, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"ok": true})
	})
	defer server.Close()

	p := NewProvider()
	require.NoError(t, p.Init(context.Background(), provider.ProviderConfig{
		Enabled: true,
		Type:    "openapi",
		Config: map[string]interface{}{
			"baseUrl": server.URL,
			"methods": []interface{}{
				map[string]interface{}{
					"name":   "uploadAvatar",
					"path":   "/avatars",
					"method": "POST",
					"requestBody": map[string]interface{}{
						"type":   "multipart",
						"fields": map[string]interface{}{"avatar": "avatar", "playerId": "playerId"},
					},
				},
			},
		},
	}))

	payload, err := json.Marshal(map[string]interface{}{
		"playerId": "p-1",
		"avatar": map[string]interface{}{
			"filename": "avatar.png",
			"content":  dataURL(sui013AvatarData, "image/png"),
		},
	})
	require.NoError(t, err)
	_, err = p.Call(context.Background(), "uploadAvatar", payload)
	require.NoError(t, err)

	assert.True(t, strings.HasPrefix(gotContentType, "multipart/form-data; boundary="),
		"the request must carry the real multipart header, got %q", gotContentType)
	boundary := strings.TrimPrefix(gotContentType, "multipart/form-data; boundary=")
	form, err := multipart.NewReader(strings.NewReader(string(gotRaw)), boundary).ReadForm(1 << 20)
	require.NoError(t, err)
	assert.Equal(t, "p-1", firstString(form.Value["playerId"]))
	require.Len(t, form.File["avatar"], 1)
	assert.Equal(t, "avatar.png", form.File["avatar"][0].Filename)
	avatarReader, err := form.File["avatar"][0].Open()
	require.NoError(t, err)
	content, err := io.ReadAll(avatarReader)
	_ = avatarReader.Close()
	require.NoError(t, err)
	assert.Equal(t, sui013AvatarData, base64.StdEncoding.EncodeToString(content),
		"the upstream must receive the decoded bytes, not the data URL")
}
