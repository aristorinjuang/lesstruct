package handlers

import (
	"context"
	"net/http"
	"strconv"

	"github.com/aristorinjuang/lesstruct/internal/api/middleware"
	dashboarddomain "github.com/aristorinjuang/lesstruct/internal/domain/dashboard"
	"github.com/aristorinjuang/lesstruct/internal/util"
)

type DashboardServiceInterface interface {
	GetStats(ctx context.Context, userID int) (*dashboarddomain.Stats, error)
}

type DashboardHandler struct {
	dashboardService DashboardServiceInterface
	logger           *util.Logger
}

func (h *DashboardHandler) GetStats(w http.ResponseWriter, r *http.Request) {
	userIDStr, ok := middleware.GetUserID(r)
	if !ok {
		sendErrorResponse(w, http.StatusUnauthorized, "unauthorized", "User not authenticated", nil)
		return
	}

	userID, err := strconv.Atoi(userIDStr)
	if err != nil {
		sendErrorResponse(w, http.StatusBadRequest, "invalid_user_id", "Invalid user ID", nil)
		return
	}

	if userID <= 0 {
		sendErrorResponse(w, http.StatusBadRequest, "invalid_user_id", "Invalid user ID", nil)
		return
	}

	stats, err := h.dashboardService.GetStats(r.Context(), userID)
	if err != nil {
		h.logger.Error("Failed to get dashboard stats: %v", err)
		sendErrorResponse(w, http.StatusInternalServerError, "internal_error", "Failed to retrieve dashboard statistics", nil)
		return
	}

	sendSuccessResponse(w, http.StatusOK, stats)
}

func NewDashboardHandler(dashboardService DashboardServiceInterface, logger *util.Logger) *DashboardHandler {
	return &DashboardHandler{
		dashboardService: dashboardService,
		logger:           logger,
	}
}
