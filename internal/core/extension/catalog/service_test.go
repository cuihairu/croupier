package catalog

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewService(t *testing.T) {
	svc := NewService(nil, nil)
	assert.NotNil(t, svc)
}

func TestService_List_NilRepo(t *testing.T) {
	svc := &Service{}
	items, total, err := svc.List(context.Background(), ListQuery{})
	assert.NoError(t, err)
	assert.Empty(t, items)
	assert.Equal(t, int64(0), total)
}

func TestService_Get_NilRepo(t *testing.T) {
	svc := &Service{}
	item, releases, err := svc.Get(context.Background(), "ext-1")
	assert.Error(t, err)
	assert.Nil(t, item)
	assert.Nil(t, releases)
}
