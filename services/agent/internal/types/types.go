package types

type (
	AgentRegisterRequest struct {
		GameId     string            `json:"game_id"`
		Env        string            `json:"env"`
		AgentId    string            `json:"agent_id"`
		RpcAddr    string            `json:"rpc_addr"`
		Ip         string            `json:"ip"`
		Type       string            `json:"type"`
		Version    string            `json:"version"`
		Functions  int64             `json:"functions"`
		Metadata   map[string]string `json:"metadata,optional"`
	}

	AgentRegisterResponse struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
		Token   string `json:"token,optional"`
	}

	AgentHeartbeatRequest struct {
		AgentId   string            `json:"agent_id"`
		GameId    string            `json:"game_id"`
		Env       string            `json:"env"`
		Functions int64             `json:"functions"`
		Status    string            `json:"status"`
		Metadata  map[string]string `json:"metadata,optional"`
	}

	AgentHeartbeatResponse struct {
		Success       bool  `json:"success"`
		NextHeartbeat int64 `json:"next_heartbeat"`
	}

	FunctionRegisterRequest struct {
		GameId     string                 `json:"game_id"`
		Env        string                 `json:"env"`
		FunctionId string                 `json:"function_id"`
		Descriptor map[string]interface{} `json:"descriptor"`
		Schema     map[string]interface{} `json:"schema,optional"`
		Metadata   map[string]interface{} `json:"metadata,optional"`
	}

	FunctionRegisterResponse struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}

	JobExecuteRequest struct {
		JobId      string                 `json:"job_id"`
		FunctionId string                 `json:"function_id"`
		GameId     string                 `json:"game_id"`
		Env        string                 `json:"env"`
		Inputs     map[string]interface{} `json:"inputs"`
		Options    map[string]interface{} `json:"options,optional"`
	}

	JobExecuteResponse struct {
		Success bool   `json:"success"`
		JobId   string `json:"job_id"`
		Status  string `json:"status"`
	}

	JobStatusRequest struct {
		JobId string `path:"job_id"`
	}

	JobStatusResponse struct {
		JobId    string                 `json:"job_id"`
		Status   string                 `json:"status"`
		Result   map[string]interface{} `json:"result,optional"`
		Error    string                 `json:"error,optional"`
		Progress int64                  `json:"progress"`
		StartTime string                `json:"start_time"`
		EndTime   string                 `json:"end_time,optional"`
	}

	AgentHealthRequest struct {
		AgentId string `path:"agent_id"`
	}

	AgentHealthResponse struct {
		Status    string  `json:"status"`
		Uptime    int64   `json:"uptime"`
		Jobs      int64   `json:"active_jobs"`
		Functions int64   `json:"functions"`
		Memory    int64   `json:"memory_usage"`
		Cpu       float64 `json:"cpu_usage"`
	}

	AgentMetricsRequest struct {
		AgentId string `path:"agent_id"`
		Start   string `form:"start,optional"`
		End     string `form:"end,optional"`
	}

	AgentMetricsResponse struct {
		Metrics map[string]interface{} `json:"metrics"`
	}
)