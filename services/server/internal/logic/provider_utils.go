package logic

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/xeipuuv/gojsonschema"
)

var errRegistryUnavailable = errors.New("registry unavailable")

func validateManifestJSON(doc []byte) error {
	schemaPath := "docs/providers-manifest.schema.json"
	if _, err := os.Stat(schemaPath); errors.Is(err, os.ErrNotExist) {
		// allow overriding via env for custom paths
		if alt := strings.TrimSpace(os.Getenv("CROUPIER_PROVIDER_SCHEMA")); alt != "" {
			schemaPath = alt
		}
	}
	data, err := os.ReadFile(schemaPath)
	if err != nil {
		return fmt.Errorf("schema not available: %w", err)
	}
	sLoader := gojsonschema.NewBytesLoader(data)
	dLoader := gojsonschema.NewBytesLoader(doc)
	res, err := gojsonschema.Validate(sLoader, dLoader)
	if err != nil {
		return err
	}
	if !res.Valid() {
		var msgs []string
		for i, e := range res.Errors() {
			if i >= 5 {
				break
			}
			msgs = append(msgs, e.String())
		}
		return fmt.Errorf("%s", strings.Join(msgs, "; "))
	}
	return nil
}
