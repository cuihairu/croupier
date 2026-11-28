package ops

import (
	"github.com/cuihairu/croupier/internal/common/response"
	"github.com/gin-gonic/gin"
)

func (h *Handler) OpsNodes(c *gin.Context) {
	var req OpsNodesRequest
	if err := bindOpsRequest(c, &req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.OpsNodes(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) OpsNodeCommands(c *gin.Context) {
	var req OpsNodeCommandsRequest
	if err := bindOpsRequest(c, &req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.OpsNodeCommands(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) OpsNodeDrain(c *gin.Context) {
	var req OpsNodeCommandsRequest
	if err := bindOpsRequest(c, &req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.OpsNodeDrain(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) OpsNodeMeta(c *gin.Context) {
	var req OpsNodeMetaRequest
	if err := bindOpsRequest(c, &req); err != nil {
		response.Error(c, err)
		return
	}
	// path-parameter fallback: BindQueryCompat only binds query, not the uri tag.
	if req.NodeID == "" {
		req.NodeID = c.Param("nodeId")
	}

	resp, err := h.service.OpsNodeMeta(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// OpsNodeDetail handles GET /ops/nodes/:nodeId — the single-device detail (the
// NodeDetail alias mounts the /nodes/:nodeId route with the same semantics as the
// meta family).
func (h *Handler) OpsNodeDetail(c *gin.Context) {
	var req OpsNodeDetailRequest
	if err := bindOpsRequest(c, &req); err != nil {
		response.Error(c, err)
		return
	}
	// path-parameter fallback: BindQueryCompat only binds query, not the uri tag.
	if req.NodeID == "" {
		req.NodeID = c.Param("nodeId")
	}

	resp, err := h.service.OpsNodeDetail(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) OpsNodeRestart(c *gin.Context) {
	var req OpsNodeCommandsRequest
	if err := bindOpsRequest(c, &req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.OpsNodeRestart(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) OpsNodeUndrain(c *gin.Context) {
	var req OpsNodeCommandsRequest
	if err := bindOpsRequest(c, &req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.OpsNodeUndrain(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// Health and maintenance handlers

func (h *Handler) Nodes(c *gin.Context) {
	h.OpsNodes(c)
}

func (h *Handler) NodeCommands(c *gin.Context) {
	h.OpsNodeCommands(c)
}

func (h *Handler) NodeDrain(c *gin.Context) {
	h.OpsNodeDrain(c)
}

func (h *Handler) NodeMeta(c *gin.Context) {
	h.OpsNodeMeta(c)
}

// NodeDetail handles GET /nodes/:nodeId (single-device detail, same source as the list).
func (h *Handler) NodeDetail(c *gin.Context) {
	h.OpsNodeDetail(c)
}

func (h *Handler) NodeRestart(c *gin.Context) {
	h.OpsNodeRestart(c)
}

func (h *Handler) NodeUndrain(c *gin.Context) {
	h.OpsNodeUndrain(c)
}
