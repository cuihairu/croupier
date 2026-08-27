package dbtype

import (
	"encoding/json"
	"testing"

	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	gmysql "gorm.io/driver/mysql"
	gpostgres "gorm.io/driver/postgres"
	gsqlserver "gorm.io/driver/sqlserver"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/schema"
)

type upperStringer struct{}

func (upperStringer) String() string { return `{"from":"stringer"}` }

type stubDialector struct{ name string }

func (s stubDialector) Name() string                                          { return s.name }
func (s stubDialector) Initialize(*gorm.DB) error                             { return nil }
func (s stubDialector) Migrator(*gorm.DB) gorm.Migrator                       { return nil }
func (s stubDialector) DataTypeOf(*schema.Field) string                       { return "" }
func (s stubDialector) DefaultValueOf(*schema.Field) clause.Expression        { return nil }
func (s stubDialector) BindVarTo(clause.Writer, *gorm.Statement, interface{}) {}
func (s stubDialector) QuoteTo(clause.Writer, string)                         {}
func (s stubDialector) Explain(sql string, _ ...interface{}) string           { return sql }

func dbWithDialector(d gorm.Dialector) *gorm.DB {
	return &gorm.DB{Config: &gorm.Config{Dialector: d}}
}

func TestJSONValue(t *testing.T) {
	v, err := JSON(nil).Value()
	require.NoError(t, err)
	assert.Nil(t, v)

	v, err = JSON(`{"a":1}`).Value()
	require.NoError(t, err)
	assert.Equal(t, `{"a":1}`, v)
}

func TestJSONScan(t *testing.T) {
	var j JSON

	require.NoError(t, j.Scan(nil))
	assert.Equal(t, "null", j.String())

	require.NoError(t, j.Scan([]byte(`{"b":2}`)))
	assert.Equal(t, `{"b":2}`, j.String())

	require.NoError(t, j.Scan(`{"c":3}`))

	assert.Equal(t, `{"c":3}`, j.String())

	require.NoError(t, j.Scan(upperStringer{}))
	assert.Equal(t, `{"from":"stringer"}`, j.String())

	require.NoError(t, j.Scan([]byte{}))
	assert.Len(t, j, 0)

	err := j.Scan(12345)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Failed to unmarshal JSONB value")
}

func TestJSONMarshalJSON(t *testing.T) {
	data, err := JSON(`{"ok":true}`).MarshalJSON()
	require.NoError(t, err)
	assert.Equal(t, `{"ok":true}`, string(data))

	data, err = JSON(nil).MarshalJSON()
	require.NoError(t, err)
	assert.Equal(t, "null", string(data))
}

func TestJSONUnmarshalJSON(t *testing.T) {
	var j JSON
	require.NoError(t, j.UnmarshalJSON([]byte(`{"x":1}`)))
	assert.Equal(t, `{"x":1}`, j.String())

	// RawMessage 语义：解析阶段不校验，原文透传。
	var k JSON
	require.NoError(t, k.UnmarshalJSON([]byte(`{invalid`)))
	assert.Equal(t, `{invalid`, k.String())

	// 嵌入结构体后经 encoding/json 往返仍保持原文。
	type wrapper struct {
		Payload JSON `json:"payload"`
	}
	w := wrapper{Payload: JSON(`[1,2]`)}
	out, err := json.Marshal(w)
	require.NoError(t, err)
	assert.JSONEq(t, `{"payload":[1,2]}`, string(out))
}

func TestJSONGormDataType(t *testing.T) {
	assert.Equal(t, "json", JSON{}.GormDataType())
}

func TestJSONGormDBDataType(t *testing.T) {
	sqliteDB, err := gorm.Open(gsqlite.Open("file:dbtype_gormdb?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)

	cases := map[string]struct {
		db   *gorm.DB
		want string
	}{
		"sqlite":    {db: sqliteDB, want: "JSON"},
		"mysql":     {db: dbWithDialector(gmysql.New(gmysql.Config{})), want: "JSON"},
		"postgres":  {db: dbWithDialector(gpostgres.New(gpostgres.Config{})), want: "JSONB"},
		"sqlserver": {db: dbWithDialector(gsqlserver.New(gsqlserver.Config{})), want: "nvarchar(max)"},
		"unknown":   {db: dbWithDialector(stubDialector{name: "oracle"}), want: "nvarchar(max)"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tc.want, JSON{}.GormDBDataType(tc.db, nil))
		})
	}
}

func TestJSONGormValue(t *testing.T) {
	expr := JSON(nil).GormValue(t.Context(), dbWithDialector(stubDialector{name: "sqlite"}))
	assert.Equal(t, "NULL", expr.SQL)

	expr = JSON(`{"k":"v"}`).GormValue(t.Context(), dbWithDialector(gpostgres.New(gpostgres.Config{})))
	assert.Equal(t, "?", expr.SQL)
	require.Len(t, expr.Vars, 1)
	assert.Equal(t, `{"k":"v"}`, expr.Vars[0])

	expr = JSON(`{"k":"v"}`).GormValue(t.Context(), dbWithDialector(gmysql.New(gmysql.Config{})))
	assert.Equal(t, "CAST(? AS JSON)", expr.SQL)

	maria := dbWithDialector(gmysql.New(gmysql.Config{}))
	maria.Dialector.(*gmysql.Dialector).ServerVersion = "11.4.3-MariaDB"
	expr = JSON(`{"k":"v"}`).GormValue(t.Context(), maria)
	assert.Equal(t, "?", expr.SQL)
}

func TestJSONString(t *testing.T) {
	assert.Equal(t, `{"a":1}`, JSON(`{"a":1}`).String())
	assert.Equal(t, "", JSON(nil).String())
}
