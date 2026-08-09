package handlers

import (
	"context"
	"net/http"
	"strconv"

	"github.com/aristorinjuang/lesstruct/internal/api/middleware"
	contentdomain "github.com/aristorinjuang/lesstruct/internal/domain/content"
	dashboarddomain "github.com/aristorinjuang/lesstruct/internal/domain/dashboard"
	"github.com/aristorinjuang/lesstruct/internal/domain/posttype"
	"github.com/aristorinjuang/lesstruct/internal/util"
)

type DashboardServiceInterface interface {
	GetStats(ctx context.Context, userID int) (*dashboarddomain.Stats, error)
}

// PostTypeLister lists registered post types. It is satisfied by the post type
// service, and lets the dashboard drop hidden post types from its per-type
// breakdown cards.
type PostTypeLister interface {
	GetAll() []posttype.PostType
}

type DashboardHandler struct {
	dashboardService  DashboardServiceInterface
	postTypeLister    PostTypeLister
	logger            *util.Logger
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

	// Admins manage the whole site: scope their stats globally (userID=0) so the
	// dashboard matches what the admin content list shows. Regular users stay
	// scoped to their own content.
	statsUserID := userID
	if role, ok := middleware.GetRole(r); ok && role == contentdomain.RoleAdmin {
		statsUserID = 0
	}

	stats, err := h.dashboardService.GetStats(r.Context(), statsUserID)
	if err != nil {
		h.logger.Error("Failed to get dashboard stats: %v", err)
		sendErrorResponse(w, http.StatusInternalServerError, "internal_error", "Failed to retrieve dashboard statistics", nil)
		return
	}

	// Drop per-type breakdown entries for hidden post types. The repository
	// GROUP BYs the content table directly, so it cannot know about the
	// presentation-level Hidden flag — filter here.
	if h.postTypeLister != nil {
		hiddenSlugs := make(map[string]bool)
		for _, pt := range h.postTypeLister.GetAll() {
			if pt.Hidden {
				hiddenSlugs[pt.Slug] = true
			}
		}
		if len(hiddenSlugs) > 0 && stats != nil {
			filtered := stats.ContentByType[:0]
			for _, entry := range stats.ContentByType {
				if !hiddenSlugs[entry.PostType] {
					filtered = append(filtered, entry)
				}
			}
			stats.ContentByType = filtered
		}
	}

	sendSuccessResponse(w, http.StatusOK, stats)
}

func NewDashboardHandler(dashboardService DashboardServiceInterface, postTypeLister PostTypeLister, logger *util.Logger) *DashboardHandler {
	return &DashboardHandler{
		dashboardService: dashboardService,
		postTypeLister:   postTypeLister,
		logger:           logger,
	}
}
