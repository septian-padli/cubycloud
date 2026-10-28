package jobs

import (
	"encoding/json"
	"net/http"

	"github.com/labstack/echo/v4"
)

type Handler struct {
	queue *Queue
}

func NewHandler(queue *Queue) *Handler {
	return &Handler{queue: queue}
}

func (h *Handler) GetJob(c echo.Context) error {
	jobID := c.Param("id")
	if jobID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "job id required"})
	}

	userID, _ := c.Get("user_id").(string)
	job, err := h.queue.Get(c.Request().Context(), jobID, "")
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	if job == nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "job not found"})
	}

	// Verify authorization: owner or superadmin
	role, _ := c.Get("role").(string)
	if job.UserID != userID && role != "SUPER_ADMIN" {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "access denied"})
	}

	resp := JobResponse{
		ID:           job.ID,
		Type:         job.Type,
		Status:       job.Status,
		UserID:       job.UserID,
		Progress:     job.Progress,
		ErrorMessage: job.ErrorMessage,
		CreatedAt:    job.CreatedAt,
		StartedAt:    job.StartedAt,
		CompletedAt:  job.CompletedAt,
	}

	if job.Result != nil && *job.Result != "" {
		var resObj any
		if err := json.Unmarshal([]byte(*job.Result), &resObj); err == nil {
			resp.Result = resObj
		} else {
			resp.Result = *job.Result
		}
	}

	return c.JSON(http.StatusOK, resp)
}

func RegisterRoutes(e *echo.Echo, queue *Queue, jwtAuth echo.MiddlewareFunc) *Handler {
	h := NewHandler(queue)
	group := e.Group("/api/v1/jobs", jwtAuth)
	group.GET("/:id", h.GetJob)
	return h
}
