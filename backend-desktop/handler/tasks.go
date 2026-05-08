package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"lingxi-agent/db"
)

// ListTasks GET /api/tasks?sessionId=xxx
func ListTasks(c *gin.Context) {
	var sessionID int64
	if sid := c.Query("sessionId"); sid != "" {
		var err error
		sessionID, err = strconv.ParseInt(sid, 10, 64)
		if err != nil {
			c.Status(http.StatusBadRequest)
			return
		}
	}

	tasks, err := db.ListTasks(sessionID)
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}
	c.JSON(http.StatusOK, tasks)
}

// DeleteTask DELETE /api/tasks/:id
func DeleteTask(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.Status(http.StatusBadRequest)
		return
	}
	db.DeleteTask(id)
	c.Status(http.StatusOK)
}
