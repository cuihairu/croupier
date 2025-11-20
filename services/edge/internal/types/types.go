package types

type (
	TunnelCreateRequest struct {
		AgentId    string                 `json:"agent_id"`
		ServerId   string                 `json:"server_id"`
		Protocol   string                 `json:"protocol"` // http, grpc, ws
		RemoteAddr string                 `json:"remote_addr"`
		LocalAddr  string                 `json:"local_addr"`
		Options    map[string]interface{} `json:"options,optional"`
	}

	TunnelCreateResponse struct {
		Success   bool   `json:"success"`
		TunnelId  string `json:"tunnel_id"`
		Message   string `json:"message"`
		PublicUrl string `json:"public_url,optional"`
	}

	TunnelStatusRequest struct {
		TunnelId string `path:"tunnel_id"`
	}

	TunnelStatusResponse struct {
		TunnelId    string `json:"tunnel_id"`
		Status      string `json:"status"`
		Protocol    string `json:"protocol"`
		RemoteAddr  string `json:"remote_addr"`
		LocalAddr   string `json:"local_addr"`
		Connections int64  `json:"connections"`
		BytesIn     int64  `json:"bytes_in"`
		BytesOut    int64  `json:"bytes_out"`
		CreatedAt   string `json:"created_at"`
		LastActive  string `json:"last_active"`
	}

	TunnelListRequest struct {
		AgentId string `form:"agent_id,optional"`
		Status  string `form:"status,optional"`
		Page    int    `form:"page,optional"`
		Size    int    `form:"size,optional"`
	}

	TunnelListResponse struct {
		Tunnels []TunnelStatusResponse `json:"tunnels"`
		Total   int64                  `json:"total"`
		Page    int                    `json:"page"`
		Size    int                    `json:"size"`
	}

	TunnelCloseRequest struct {
		TunnelId string `path:"tunnel_id"`
	}

	TunnelCloseResponse struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}

	ProxyRequest struct {
		TunnelId string            `json:"tunnel_id"`
		Method   string            `json:"method"`
		Path     string            `json:"path"`
		Headers  map[string]string `json:"headers,optional"`
		Body     string            `json:"body,optional"`
	}

	ProxyResponse struct {
		Status  int               `json:"status"`
		Headers map[string]string `json:"headers"`
		Body    string            `json:"body"`
	}

	EdgeHealthRequest struct {
		RequestId string `path:"request_id"`
	}

	EdgeHealthResponse struct {
		Status    string            `json:"status"`
		Uptime    int64             `json:"uptime"`
		Version   string            `json:"version"`
		Tunnels   int64             `json:"active_tunnels"`
		Agents    int64             `json:"connected_agents"`
		Load      map[string]float64 `json:"load"`
		Timestamp string            `json:"timestamp"`
	}

	EdgeMetricsRequest struct {
		Start string `form:"start,optional"`
		End   string `form:"end,optional"`
		Type  string `form:"type,optional"` // system, tunnels, agents
	}

	EdgeMetricsResponse struct {
		Metrics map[string]interface{} `json:"metrics"`
	}
)