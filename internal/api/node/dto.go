// Package node provides DTOs for node-related API operations.
package node

// Node represents a node in the system.
type Node struct {
	ID        string      `json:"id"`
	Name      string      `json:"name"`
	Type      string      `json:"type"` // server, agent
	Status    string      `json:"status"`
	IP        string      `json:"ip"`
	Port      int         `json:"port"`
	Resources interface{} `json:"resources"`
	UpdatedAt string      `json:"updatedAt"`
}

// NodeActionRequest represents a request for node action.
type NodeActionRequest struct {
	ID string `uri:"id"`
}

// NodeCommand represents a command that can be executed on a node.
type NodeCommand struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// NodeCommandsRequest represents a request to list node commands.
type NodeCommandsRequest struct {
}

// NodeCommandsResponse represents the response for listing node commands.
type NodeCommandsResponse struct {
	Items []NodeCommand `json:"items"`
}

// NodeDrainRequest represents a request to drain a node.
type NodeDrainRequest struct {
	ID      string `uri:"id"`
	Timeout int    `json:"timeout"` // 秒
}

// NodeMetaRequest represents a request to get node metadata.
type NodeMetaRequest struct {
	ID string `uri:"id"`
}

// NodeMetaResponse represents the response for getting node metadata.
type NodeMetaResponse struct {
	Meta interface{} `json:"meta"`
}

// NodeMetaUpdateRequest represents a request to update node metadata.
type NodeMetaUpdateRequest struct {
	ID   string      `uri:"id"`
	Meta interface{} `json:"meta"`
}

// NodesListRequest represents a request to list nodes.
type NodesListRequest struct {
	Type   string `form:"type"`
	Status string `form:"status"`
}

// NodesListResponse represents the response for listing nodes.
type NodesListResponse struct {
	Items []Node `json:"items"`
}

// ObjectInfo represents information about a storage object.
type ObjectInfo struct {
	Key          string `json:"key"`
	Size         int64  `json:"size"`
	LastModified string `json:"last_modified"`
	ETag         string `json:"etag"`
	StorageClass string `json:"storage_class"`
}

// ObjectsData represents a paginated list of storage objects.
type ObjectsData struct {
	Objects     []ObjectInfo `json:"objects"`
	Prefixes    []string     `json:"prefixes"`
	IsTruncated bool         `json:"is_truncated"`
	NextMarker  string       `json:"next_marker"`
}
