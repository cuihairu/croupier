// Coverage sweep round 21 (wt-api): the bindOpsRequest error wing of
// handler.go OpsNodeDetail (352-354) is registered as defensive unreachable —
// OpsNodeDetailRequest has only a single optional string field (form + uri
// tag, no binding validation constraint), and on the GET side
// BindQueryCompat's query-compatibility binding has no failure path (proven
// isomorphic in round 4 by the provider SdkStats). This file locks the
// "binding any query never fails" premise with a proving test: if a future
// field introduces required/strong typing, the branch becomes reachable and
// real error-path cases must be added then.
package ops

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOpsNodeDetailBindNeverFails(t *testing.T) {
	for _, q := range []string{
		"",
		"?nodeId=%zz",
		"?nodeId=<script>",
		"?nodeId=a&nodeId=b",
	} {
		ctx, _ := newOpsTestContext(http.MethodGet, "/api/v1/ops/nodes"+q, "")
		var req OpsNodeDetailRequest
		require.NoError(t, bindOpsRequest(ctx, &req), "query %q 不应产生绑定错误（分支不可达前提）", q)
	}
}
