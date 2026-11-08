package protocol

// message.go 补测：SupportedVersions / IsSupportedVersion 的版本窗口判断、
// MsgIDString 对 supervisor 日志 / 告警事件 / capture 配置六条消息名的
// 未触达臂。
//
// 无诚实跳过：纯函数，全臂可达。

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSupportedVersionsWindow(t *testing.T) {
	require.Equal(t, []int{int(Version1)}, SupportedVersions())
	require.True(t, IsSupportedVersion(Version1))
	require.True(t, IsSupportedVersion(MinSupportedVersion))
	require.False(t, IsSupportedVersion(0))
	require.False(t, IsSupportedVersion(Version1+1))
}

func TestMsgIDStringLateOpsArms(t *testing.T) {
	for _, msgID := range []uint32{
		MsgGetSupervisorLogRequest, MsgGetSupervisorLogResponse,
		MsgAlertEventRequest, MsgAlertEventResponse,
		MsgCaptureConfigRequest, MsgCaptureConfigResponse,
	} {
		require.NotEqual(t, "Unknown", MsgIDString(msgID), "msgID %#x", msgID)
	}
	require.Equal(t, "GetSupervisorLogRequest", MsgIDString(MsgGetSupervisorLogRequest))
	require.Equal(t, "AlertEventResponse", MsgIDString(MsgAlertEventResponse))
}
