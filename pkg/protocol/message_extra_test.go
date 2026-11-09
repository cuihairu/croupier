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
	versions := SupportedVersions()
	// 当前窗口退化为 [1,1]，清单恰为 [Version1]；窗口开启（N-1）后派生
	// 逻辑必须报告全部受支持版本而非只报当前版。
	require.Equal(t, []int{int(Version1)}, versions)
	// 派生性质：降序、每个版本都落在接受窗口内。
	require.Greater(t, len(versions), 0)
	for i, v := range versions {
		require.True(t, IsSupportedVersion(uint8(v)), "version %d not in window", v)
		if i > 0 {
			require.Less(t, versions[i], versions[i-1], "versions must be sorted descending")
		}
	}
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
