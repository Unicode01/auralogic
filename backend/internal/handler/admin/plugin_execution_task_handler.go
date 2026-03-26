package admin

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

func parsePluginExecutionTaskLimit(c *gin.Context, defaultValue int) int {
	if defaultValue <= 0 {
		defaultValue = 50
	}
	if c == nil {
		return defaultValue
	}

	raw := strings.TrimSpace(c.Query("limit"))
	if raw == "" {
		return defaultValue
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return defaultValue
	}
	if value > 200 {
		return 200
	}
	return value
}

func (h *PluginHandler) GetPluginExecutionTasks(c *gin.Context) {
	id, ok := h.parsePluginID(c)
	if !ok {
		return
	}
	if h == nil || h.pluginManager == nil {
		h.respondPluginError(c, http.StatusServiceUnavailable, "Plugin manager is unavailable")
		return
	}

	status := strings.ToLower(strings.TrimSpace(c.Query("status")))
	if status == "" {
		status = "all"
	}
	limit := parsePluginExecutionTaskLimit(c, 50)
	tasks := h.pluginManager.ListPluginExecutionTasks(id, status, limit)
	overview := h.pluginManager.InspectPluginExecutionTasks(id, limit, limit)
	c.JSON(http.StatusOK, gin.H{
		"plugin_id":    id,
		"status":       status,
		"limit":        limit,
		"tasks":        tasks,
		"active_count": overview.ActiveCount,
		"recent_count": overview.RecentCount,
		"active_tasks": overview.Active,
		"recent_tasks": overview.Recent,
	})
}

func (h *PluginHandler) GetPluginExecutionTask(c *gin.Context) {
	id, ok := h.parsePluginID(c)
	if !ok {
		return
	}
	if h == nil || h.pluginManager == nil {
		h.respondPluginError(c, http.StatusServiceUnavailable, "Plugin manager is unavailable")
		return
	}

	taskID := strings.TrimSpace(c.Param("task_id"))
	if taskID == "" {
		h.respondPluginError(c, http.StatusBadRequest, "Task id is required")
		return
	}

	task, exists := h.pluginManager.GetPluginExecutionTask(id, taskID)
	if !exists || task == nil {
		h.respondPluginError(c, http.StatusNotFound, "Plugin execution task not found")
		return
	}
	c.JSON(http.StatusOK, task)
}

func (h *PluginHandler) CancelPluginExecutionTask(c *gin.Context) {
	id, ok := h.parsePluginID(c)
	if !ok {
		return
	}
	if h == nil || h.pluginManager == nil {
		h.respondPluginError(c, http.StatusServiceUnavailable, "Plugin manager is unavailable")
		return
	}

	taskID := strings.TrimSpace(c.Param("task_id"))
	if taskID == "" {
		h.respondPluginError(c, http.StatusBadRequest, "Task id is required")
		return
	}

	task, err := h.pluginManager.CancelPluginExecutionTask(id, taskID)
	if err != nil {
		status := http.StatusConflict
		if strings.Contains(strings.ToLower(err.Error()), "not found") {
			status = http.StatusNotFound
		}
		h.respondPluginError(c, status, err.Error())
		return
	}

	resourceID := id
	h.logPluginOperation(c, "plugin_execution_task_cancel", nil, &resourceID, map[string]interface{}{
		"success":        true,
		"task_id":        taskID,
		"cancel_request": true,
	})
	c.JSON(http.StatusOK, gin.H{
		"success":          true,
		"cancel_requested": true,
		"task":             task,
	})
}
