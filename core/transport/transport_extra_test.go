package transport

// HandlerFunc 适配器补测（包内唯一可执行语句）：闭包透传参数与返回值。
//
// 诚实跳过：无——接口与常量声明无执行面。

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHandlerFuncHandle(t *testing.T) {
	var gotMsgID, gotReqID uint32
	var gotBody string
	fn := HandlerFunc(func(ctx context.Context, msgID uint32, reqID uint32, body []byte) ([]byte, error) {
		gotMsgID, gotReqID, gotBody = msgID, reqID, string(body)
		return []byte("pong"), nil
	})

	resp, err := fn.Handle(context.Background(), 7, 42, []byte("ping"))
	require.NoError(t, err)
	require.Equal(t, "pong", string(resp))
	require.Equal(t, uint32(7), gotMsgID)
	require.Equal(t, uint32(42), gotReqID)
	require.Equal(t, "ping", gotBody)

	fail := HandlerFunc(func(context.Context, uint32, uint32, []byte) ([]byte, error) {
		return nil, errors.New("boom")
	})
	_, err = fail.Handle(context.Background(), 0, 0, nil)
	require.ErrorContains(t, err, "boom")
}
