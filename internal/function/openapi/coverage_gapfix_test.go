package openapi

import (
	"errors"
	"fmt"
	"testing"

	functionv1 "github.com/cuihairu/croupier/pkg/pb/croupier/function/v1"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newSpecWithPost(path, operationID string) *openapi3.T {
	spec := &openapi3.T{
		OpenAPI: "3.0.3",
		Info:    &openapi3.Info{Title: "T", Version: "1"},
		Paths:   openapi3.NewPaths(),
	}
	spec.Paths.Set(path, &openapi3.PathItem{
		Post: &openapi3.Operation{OperationID: operationID},
	})
	return spec
}

// kin-openapi v0.144 的 Operations() 永不返回 nil 指针，
// ImportFromSpec 的 "op == nil" 跳过分支只能通过缝隙注入触达。
func TestConverterImportFromSpecNilOperationSkipped(t *testing.T) {
	orig := pathItemOperations
	pathItemOperations = func(pathItem *openapi3.PathItem) map[string]*openapi3.Operation {
		return map[string]*openapi3.Operation{
			"GET": nil,
			"POST": &openapi3.Operation{
				OperationID: "real.op",
				Summary:     "Real Op",
			},
		}
	}
	t.Cleanup(func() { pathItemOperations = orig })

	metadatas, err := NewConverter().ImportFromSpec(newSpecWithPost("/p", "real.op"), nil)
	require.NoError(t, err)
	require.Len(t, metadatas, 1, "nil operation must be skipped, valid one kept")
	assert.Equal(t, "real.op", metadatas[0].Id)
}

// operationToMetadata 恒返回 nil error，ImportFromSpec 的错误分支只能
// 通过缝隙注入转换失败触达：默认（无 ContinueOnError）必须整单报错。
func TestConverterImportFromSpecConvertError(t *testing.T) {
	orig := convertOperation
	convertOperation = func(c *Converter, path string, op *openapi3.Operation, options *ImportOptions) (*functionv1.FunctionMetadata, error) {
		return nil, errors.New("injected conversion failure")
	}
	t.Cleanup(func() { convertOperation = orig })

	spec := newSpecWithPost("/p", "bad.op")
	_, err := NewConverter().ImportFromSpec(spec, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "convert operation bad.op failed")
	assert.Contains(t, err.Error(), "injected conversion failure")
}

// ContinueOnError=true 时失败的 operation 被跳过，其余继续导入。
func TestConverterImportFromSpecContinueOnError(t *testing.T) {
	orig := convertOperation
	convertOperation = func(c *Converter, path string, op *openapi3.Operation, options *ImportOptions) (*functionv1.FunctionMetadata, error) {
		if op.OperationID == "bad.op" {
			return nil, fmt.Errorf("injected conversion failure")
		}
		return c.operationToMetadata(path, op, options)
	}
	t.Cleanup(func() { convertOperation = orig })

	spec := newSpecWithPost("/p", "bad.op")
	spec.Paths.Set("/good", &openapi3.PathItem{
		Post: &openapi3.Operation{OperationID: "good.op"},
	})

	metadatas, err := NewConverter().ImportFromSpec(spec, &ImportOptions{ContinueOnError: true})
	require.NoError(t, err)
	require.Len(t, metadatas, 1, "failed op skipped, valid op imported")
	assert.Equal(t, "good.op", metadatas[0].Id)
}

// metadataToOperation 恒返回非 nil，ExportToSpec 的 "op == nil" 跳过分支
// 只能通过缝隙注入触达：nil 操作被跳过且不产生 path。
func TestConverterExportToSpecNilOperationSkipped(t *testing.T) {
	orig := buildOperation
	buildOperation = func(c *Converter, metadata *functionv1.FunctionMetadata) *openapi3.Operation {
		return nil
	}
	t.Cleanup(func() { buildOperation = orig })

	doc, err := NewConverter().ExportToSpec([]*functionv1.FunctionMetadata{
		{Id: "f1", Name: "F1"},
		{Id: "f2", Name: "F2"},
	})
	require.NoError(t, err)
	assert.Equal(t, 0, doc.Paths.Len(), "nil operations must not create paths")
}

// MetadataToOperation 对缝隙注入的 nil 结果必须报错而不是返回 nil op。
func TestConverterMetadataToOperationNilResultError(t *testing.T) {
	orig := buildOperation
	buildOperation = func(c *Converter, metadata *functionv1.FunctionMetadata) *openapi3.Operation {
		return nil
	}
	t.Cleanup(func() { buildOperation = orig })

	op, err := NewConverter().MetadataToOperation(&functionv1.FunctionMetadata{Id: "f1"})
	require.Error(t, err)
	assert.Nil(t, op)
	assert.Equal(t, "failed to convert metadata to operation", err.Error())
}

// 缝隙还原后导出/导入 round-trip 正常。
func TestConverterSeamsRestoredRoundTrip(t *testing.T) {
	origBuild := buildOperation
	buildOperation = func(c *Converter, metadata *functionv1.FunctionMetadata) *openapi3.Operation {
		return nil
	}
	t.Cleanup(func() { buildOperation = origBuild })

	NewConverter().ExportToSpec([]*functionv1.FunctionMetadata{{Id: "x", Name: "X"}})

	buildOperation = origBuild

	doc, err := NewConverter().ExportToSpec([]*functionv1.FunctionMetadata{{Id: "x", Name: "X"}})
	require.NoError(t, err)
	require.Equal(t, 1, doc.Paths.Len())

	back, err := NewConverter().ImportFromSpec(doc, nil)
	require.NoError(t, err)
	require.Len(t, back, 1)
	assert.Equal(t, "x", back[0].Id)
}

// merged map 只含 JSON 标量/容器，json.Marshal 恒成功；MergeSchemas 的
// 序列化错误分支只能通过缝隙注入触达。
func TestSchemaMapperMergeSchemasMarshalFailure(t *testing.T) {
	orig := marshalMergedSchema
	marshalMergedSchema = func(v interface{}) ([]byte, error) {
		return nil, errors.New("injected marshal failure")
	}
	t.Cleanup(func() { marshalMergedSchema = orig })

	mapper := NewSchemaMapper()
	merged, err := mapper.MergeSchemas(`{"type":"object"}`, `{"properties":{"a":{"type":"string"}}}`)
	require.Error(t, err)
	assert.Empty(t, merged)
	assert.Contains(t, err.Error(), "marshal merged schema failed")
	assert.Contains(t, err.Error(), "injected marshal failure")
}
