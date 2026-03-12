// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package certificate

import (
	"net/http"

	"github.com/cuihairu/croupier/services/server/internal/logic/certificate"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

// 检查证书状态
func CertificateCheckHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.CertificateCheckRequest
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := certificate.NewCertificateCheckLogic(r.Context(), svcCtx)
		resp, err := l.CertificateCheck(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
