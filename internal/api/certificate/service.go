package certificate

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
)

type Service struct {
	svcCtx *svc.ServiceContext
}

func NewService(svcCtx *svc.ServiceContext) *Service {
	return &Service{svcCtx: svcCtx}
}

// List returns a list of certificates
func (s *Service) List(ctx context.Context, req *ListRequest) (*ListResponse, error) {
	opts := model.ListCertificatesOptions{
		Page:     req.Page,
		PageSize: req.PageSize,
		Status:   strings.TrimSpace(req.Status),
	}

	certs, total, err := s.svcCtx.CertificateModel.List(ctx, opts)
	if err != nil {
		return nil, err
	}

	items := make([]CertificateItem, 0, len(certs))
	for i := range certs {
		items = append(items, BuildCertificateDTO(&certs[i]))
	}

	return &ListResponse{
		Items: items,
		Total: total,
		Page:  req.Page,
		Size:  req.PageSize,
	}, nil
}

// Add adds a new certificate
func (s *Service) Add(ctx context.Context, req *AddRequest) (*AddResponse, error) {
	domain, err := utils.ValidateDomain(req.Domain)
	if err != nil {
		return nil, err
	}

	certPEM := strings.TrimSpace(req.Certificate)
	if certPEM == "" {
		return nil, errorx.NewBadRequest("证书内容不能为空")
	}

	parsed, err := ParseCertificatePEM(certPEM)
	if err != nil {
		return nil, err
	}

	certificate := &model.Certificate{
		Domain:         domain,
		CertificatePEM: certPEM,
		PrivateKeyPEM:  strings.TrimSpace(req.PrivateKey),
		Issuer:         FormatIssuer(parsed),
		ExpiresAt:      parsed.NotAfter,
		Status:         model.CertificateStatus(parsed.NotAfter),
	}

	if err := s.svcCtx.CertificateModel.Create(ctx, certificate); err != nil {
		return nil, err
	}

	return &AddResponse{
		Certificate: BuildCertificateDTO(certificate),
	}, nil
}

// Get returns certificate details
func (s *Service) Get(ctx context.Context, req *GetRequest) (*GetResponse, error) {
	id, err := utils.ParseUintID(req.ID, "证书ID")
	if err != nil {
		return nil, err
	}

	cert, err := s.svcCtx.CertificateModel.FindOne(ctx, id)
	if err != nil {
		return nil, err
	}

	return &GetResponse{
		Certificate: BuildCertificateDTO(cert),
	}, nil
}

// Check checks certificate status
func (s *Service) Check(ctx context.Context, req *CheckRequest) (*CheckResponse, error) {
	id, err := utils.ParseUintID(req.ID, "证书ID")
	if err != nil {
		return nil, err
	}

	cert, err := s.svcCtx.CertificateModel.FindOne(ctx, id)
	if err != nil {
		return nil, err
	}

	parsed, err := ParseCertificatePEM(cert.CertificatePEM)
	if err != nil {
		return nil, err
	}

	cert.ExpiresAt = parsed.NotAfter
	cert.Issuer = FormatIssuer(parsed)
	UpdateCertificateStatus(cert)

	if err := s.svcCtx.CertificateModel.Update(ctx, cert.ID, map[string]interface{}{
		"expires_at":      cert.ExpiresAt,
		"issuer":          cert.Issuer,
		"status":          cert.Status,
		"last_checked_at": cert.LastCheckedAt,
		"error_message":   cert.ErrorMessage,
	}); err != nil {
		return nil, err
	}

	return &CheckResponse{
		Certificate: BuildCertificateDTO(cert),
	}, nil
}

// Delete deletes a certificate
func (s *Service) Delete(ctx context.Context, req *DeleteRequest) error {
	id, err := utils.ParseUintID(req.ID, "证书ID")
	if err != nil {
		return err
	}

	if err := s.svcCtx.CertificateModel.Delete(ctx, id); err != nil {
		return err
	}
	return nil
}

// Stats returns certificate statistics
func (s *Service) Stats(ctx context.Context) (*StatsResponse, error) {
	stats, err := s.svcCtx.CertificateModel.Stats(ctx)
	if err != nil {
		return nil, err
	}

	return &StatsResponse{
		Total:    stats["total"],
		Valid:    stats["valid"],
		Expiring: stats["expiring"],
		Expired:  stats["expired"],
		Invalid:  stats["invalid"],
	}, nil
}

// AddAlert adds a certificate alert
func (s *Service) AddAlert(ctx context.Context, req *AddAlertRequest) (*AddAlertResponse, error) {
	domain, err := utils.ValidateDomain(req.Domain)
	if err != nil {
		return nil, err
	}

	if _, err := s.svcCtx.CertificateModel.FindByDomain(ctx, domain); err != nil {
		return nil, err
	}

	threshold := req.Threshold
	if threshold <= 0 {
		threshold = 30
	}

	alert := &model.CertificateAlert{
		Domain:        domain,
		ThresholdDays: threshold,
		Active:        true,
	}

	if err := s.svcCtx.CertificateModel.AddAlert(ctx, alert); err != nil {
		return nil, err
	}

	return &AddAlertResponse{
		ID:            alert.ID,
		Domain:        alert.Domain,
		ThresholdDays: alert.ThresholdDays,
	}, nil
}

// ListAlerts returns a list of certificate alerts
func (s *Service) ListAlerts(ctx context.Context, req *ListAlertsRequest) (*ListAlertsResponse, error) {
	alerts, total, err := s.svcCtx.CertificateModel.ListAlerts(ctx, req.Page, req.PageSize)
	if err != nil {
		return nil, err
	}

	items := make([]AlertItem, 0, len(alerts))
	for _, alert := range alerts {
		items = append(items, AlertItem{
			ID:              alert.ID,
			Domain:          alert.Domain,
			ThresholdDays:   alert.ThresholdDays,
			Active:          alert.Active,
			LastTriggeredAt: utils.FormatTimestampPtr(alert.LastTriggeredAt),
			CreatedAt:       utils.FormatTimestamp(alert.CreatedAt),
		})
	}

	return &ListAlertsResponse{
		Items: items,
		Total: total,
		Page:  req.Page,
		Size:  req.PageSize,
	}, nil
}

// CheckAll checks all certificates
func (s *Service) CheckAll(ctx context.Context) (*CheckAllResponse, error) {
	certs, err := s.svcCtx.CertificateModel.ListAll(ctx)
	if err != nil {
		return nil, err
	}

	var success int
	var failed int
	for i := range certs {
		cert := &certs[i]
		parsed, parseErr := ParseCertificatePEM(cert.CertificatePEM)
		if parseErr != nil {
			cert.ErrorMessage = parseErr.Error()
			cert.Status = "invalid"
			failed++
		} else {
			cert.ExpiresAt = parsed.NotAfter
			cert.Issuer = FormatIssuer(parsed)
			UpdateCertificateStatus(cert)
			success++
		}
		_ = s.svcCtx.CertificateModel.Update(ctx, cert.ID, map[string]interface{}{
			"expires_at":      cert.ExpiresAt,
			"issuer":          cert.Issuer,
			"status":          cert.Status,
			"last_checked_at": cert.LastCheckedAt,
			"error_message":   cert.ErrorMessage,
		})
	}

	return &CheckAllResponse{
		Checked: success,
		Failed:  failed,
		Total:   len(certs),
	}, nil
}

// GetDomainInfo returns certificate info for a domain
func (s *Service) GetDomainInfo(ctx context.Context, req *DomainInfoRequest) (*DomainInfoResponse, error) {
	domain, err := utils.ValidateDomain(req.Domain)
	if err != nil {
		return nil, err
	}

	cert, err := s.svcCtx.CertificateModel.FindByDomain(ctx, domain)
	if err != nil {
		return nil, err
	}

	return &DomainInfoResponse{
		Certificate: BuildCertificateDTO(cert),
	}, nil
}

// GetExpiring returns certificates expiring within specified days
func (s *Service) GetExpiring(ctx context.Context, req *ExpiringRequest) (*ExpiringResponse, error) {
	days := req.Days
	if days <= 0 {
		days = 30
	}

	certs, err := s.svcCtx.CertificateModel.ExpiringWithin(ctx, time.Hour*24*time.Duration(days))
	if err != nil {
		return nil, err
	}

	items := make([]CertificateItem, 0, len(certs))
	for i := range certs {
		items = append(items, BuildCertificateDTO(&certs[i]))
	}

	return &ExpiringResponse{
		Items: items,
		Days:  days,
	}, nil
}

// parseUintID is a helper to parse uint IDs
func parseUintID(idStr, fieldName string) (uint, error) {
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		return 0, errorx.NewBadRequest(fieldName + " must be a valid number")
	}
	return uint(id), nil
}
