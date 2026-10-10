package management

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// DeleteAuthFileSessionAffinity unpins every session bound to one auth file. Those sessions
// move to the highest-priority available credential on their next request, while the auth
// stays enabled and keeps serving new bindings by priority.
func (h *Handler) DeleteAuthFileSessionAffinity(c *gin.Context) {
	if h.authManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "core auth manager unavailable"})
		return
	}
	name := strings.TrimSpace(c.Query("name"))
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name is required"})
		return
	}
	auth, ok := h.lookupAuthFile(name, c.Query("auth_index"))
	if !ok || auth == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "auth file not found"})
		return
	}
	h.authManager.InvalidateSessionAffinity(auth.ID)
	c.JSON(http.StatusOK, gin.H{"status": "ok", "id": auth.ID})
}
