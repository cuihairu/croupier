package routes

// Routes related types

type GetRoutesResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    []RouteItem `json:"data"`
}

type RouteItem struct {
	Path      string                 `json:"path"`
	Name      string                 `json:"name"`
	Icon      string                 `json:"icon"`
	Component string                 `json:"component"`
	Routes    []RouteItem            `json:"routes,omitempty"`
	Meta      map[string]interface{} `json:"meta,omitempty"`
}
