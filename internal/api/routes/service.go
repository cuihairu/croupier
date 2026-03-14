package routes

import "context"

type Service struct{}

func NewService() *Service {
	return &Service{}
}

// GetRoutes returns the available routes
func (s *Service) GetRoutes(ctx context.Context) (*GetRoutesResponse, error) {
	routes := []RouteItem{
		{Path: "/api/v1/admin", Name: "Admin", Icon: "user", Component: "Admin"},
		{Path: "/api/v1/functions", Name: "Functions", Icon: "function", Component: "Functions"},
		{Path: "/api/v1/games", Name: "Games", Icon: "game", Component: "Games"},
		{Path: "/api/v1/jobs", Name: "Jobs", Icon: "job", Component: "Jobs"},
		{Path: "/api/v1/nodes", Name: "Nodes", Icon: "node", Component: "Nodes"},
		{Path: "/api/v1/ops", Name: "Ops", Icon: "ops", Component: "Ops"},
		{Path: "/api/v1/storage", Name: "Storage", Icon: "storage", Component: "Storage"},
		{Path: "/api/v1/agent", Name: "Agent", Icon: "agent", Component: "Agent"},
		{Path: "/api/v1/alerts", Name: "Alerts", Icon: "alert", Component: "Alerts"},
		{Path: "/api/v1/analytics", Name: "Analytics", Icon: "analytics", Component: "Analytics"},
		{Path: "/api/v1/approvals", Name: "Approvals", Icon: "approval", Component: "Approvals"},
		{Path: "/api/v1/assignments", Name: "Assignments", Icon: "assignment", Component: "Assignments"},
		{Path: "/api/v1/backups", Name: "Backups", Icon: "backup", Component: "Backups"},
		{Path: "/api/v1/certificates", Name: "Certificates", Icon: "certificate", Component: "Certificates"},
		{Path: "/api/v1/components", Name: "Components", Icon: "component", Component: "Components"},
		{Path: "/api/v1/configs", Name: "Configs", Icon: "config", Component: "Configs"},
		{Path: "/api/v1/entities", Name: "Entities", Icon: "entity", Component: "Entities"},
		{Path: "/api/v1/faqs", Name: "FAQs", Icon: "faq", Component: "FAQs"},
		{Path: "/api/v1/feedback", Name: "Feedback", Icon: "feedback", Component: "Feedback"},
		{Path: "/api/v1/messages", Name: "Messages", Icon: "message", Component: "Messages"},
		{Path: "/api/v1/packs", Name: "Packs", Icon: "pack", Component: "Packs"},
		{Path: "/api/v1/platforms", Name: "Platforms", Icon: "platform", Component: "Platforms"},
		{Path: "/api/v1/players", Name: "Players", Icon: "player", Component: "Players"},
		{Path: "/api/v1/profile", Name: "Profile", Icon: "profile", Component: "Profile"},
		{Path: "/api/v1/providers", Name: "Providers", Icon: "provider", Component: "Providers"},
		{Path: "/api/v1/rate-limits", Name: "Rate Limits", Icon: "rate-limit", Component: "RateLimits"},
		{Path: "/api/v1/schemas", Name: "Schemas", Icon: "schema", Component: "Schemas"},
		{Path: "/api/v1/terms", Name: "Terms", Icon: "terms", Component: "Terms"},
		{Path: "/api/v1/tickets", Name: "Tickets", Icon: "ticket", Component: "Tickets"},
		{Path: "/api/v1/workspaces", Name: "Workspaces", Icon: "workspace", Component: "Workspaces"},
	}

	return &GetRoutesResponse{
		Code:    0,
		Message: "OK",
		Data:    routes,
	}, nil
}
