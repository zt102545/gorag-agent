package handler

import (
	"github.com/gin-gonic/gin"

	"github.com/yourorg/agent_customer_service/internal/session"
	"github.com/yourorg/agent_customer_service/pkg/response"
)

type SessionHandler struct {
	store session.Store
}

func NewSessionHandler(store session.Store) *SessionHandler {
	return &SessionHandler{store: store}
}

func (h *SessionHandler) Create(c *gin.Context) {
	meta, err := h.store.Create(c.Request.Context())
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, meta)
}

func (h *SessionHandler) Get(c *gin.Context) {
	meta, err := h.store.Get(c.Request.Context(), c.Param("id"))
	if err != nil {
		response.NotFound(c, err.Error())
		return
	}
	response.OK(c, meta)
}

func (h *SessionHandler) History(c *gin.Context) {
	messages, err := h.store.GetMessages(c.Request.Context(), c.Param("id"))
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, gin.H{"messages": messages})
}

func (h *SessionHandler) Delete(c *gin.Context) {
	if err := h.store.Delete(c.Request.Context(), c.Param("id")); err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, gin.H{"deleted": true})
}
