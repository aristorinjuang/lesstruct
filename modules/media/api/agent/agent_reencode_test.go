package agent_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aristorinjuang/lesstruct/kernel"
	mediaagent "github.com/aristorinjuang/lesstruct/modules/media/api/agent"
	agentmocks "github.com/aristorinjuang/lesstruct/modules/media/api/agent/mocks"
	mediadomain "github.com/aristorinjuang/lesstruct/modules/media/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestMediaHandler_Reencode(t *testing.T) {
	tests := []struct {
		name       string
		userID     string
		roles      []string
		setup      func(svc *agentmocks.MockMediaService)
		wantStatus int
		wantCode   string
	}{
		{
			name:   "success - admin re-encodes library and returns summary",
			userID: "42",
			roles:  []string{"Admin"},
			setup: func(svc *agentmocks.MockMediaService) {
				svc.EXPECT().ReencodeLibrary(mock.Anything).Return(mediadomain.ReencodeSummary{Processed: 3, Skipped: 1, BytesBefore: 1000, BytesAfter: 100}, nil)
			},
			wantStatus: http.StatusOK,
		},
		{
			name:       "error - unauthenticated returns UNAUTHORIZED",
			userID:     "",
			roles:      nil,
			setup:      nil,
			wantStatus: http.StatusUnauthorized,
			wantCode:   "UNAUTHORIZED",
		},
		{
			name:       "error - non-admin returns FORBIDDEN",
			userID:     "42",
			roles:      []string{"Editor"},
			setup:      nil,
			wantStatus: http.StatusForbidden,
			wantCode:   "FORBIDDEN",
		},
		{
			name:   "success - admin in any role position is authorized",
			userID: "42",
			roles:  []string{"Editor", "Admin"},
			setup: func(svc *agentmocks.MockMediaService) {
				svc.EXPECT().ReencodeLibrary(mock.Anything).Return(mediadomain.ReencodeSummary{Processed: 1, Skipped: 0, BytesBefore: 100, BytesAfter: 10}, nil).Once()
			},
			wantStatus: http.StatusOK,
		},
		{
			name:   "error - service failure maps to INTERNAL_ERROR",
			userID: "42",
			roles:  []string{"Admin"},
			setup: func(svc *agentmocks.MockMediaService) {
				svc.EXPECT().ReencodeLibrary(mock.Anything).Return(mediadomain.ReencodeSummary{}, errors.New("db down"))
			},
			wantStatus: http.StatusInternalServerError,
			wantCode:   "INTERNAL_ERROR",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := agentmocks.NewMockMediaService(t)
			if tt.setup != nil {
				tt.setup(svc)
			}
			handler := mediaagent.NewMediaHandler(svc, nil)
			r := httptest.NewRequest(http.MethodPost, "/api/v1/media/reencode", nil)
			if tt.userID != "" {
				ctx := kernel.ContextWithPrincipal(r.Context(), kernel.Principal{
					ID:       tt.userID,
					Username: "testuser",
					Roles:    tt.roles,
				})
				r = r.WithContext(ctx)
			}
			w := httptest.NewRecorder()
			handler.Reencode(w, r)

			assert.Equal(t, tt.wantStatus, w.Code)
			if tt.wantCode != "" {
				assert.Equal(t, tt.wantCode, envelopeError(t, w))
			}
			if tt.wantStatus == http.StatusOK {
				assert.Contains(t, w.Body.String(), "processed")
				assert.Contains(t, w.Body.String(), "bytesBefore")
			}
		})
	}
}
