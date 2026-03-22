package routes

// RouteItem represents a single route in the system
type RouteItem struct {
	Path      string                 `json:"path"`
	Name      string                 `json:"name"`
	Icon      string                 `json:"icon"`
	Component string                 `json:"component"`
	Meta      map[string]interface{} `json:"meta"`
}

// GetRoutesResponse returns the available routes.
type GetRoutesResponse []RouteItem
