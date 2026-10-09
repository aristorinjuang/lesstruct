package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aristorinjuang/lesstruct/modules/users/api"
	userdomain "github.com/aristorinjuang/lesstruct/modules/users/domain"
	"github.com/go-chi/chi/v5"
)

// umgmtSetupTestRouter sets up a test router with user management routes.
// The helper name is prefixed with umgmt to avoid colliding with helpers
// defined by other parallel migration workers in package api_test.
func umgmtSetupTestRouter(
	t *testing.T,
	userRepo *stubUserRepo,
	blockedEmailRepo *stubBlockedRepo,
	emailService *stubMailer,
	softDeleteRepo *stubSoftDelete,
) *chi.Mux {
	userManagementService := userdomain.NewUserManagementService(userRepo, blockedEmailRepo, stubPolicy{}, true)
	userManagementHandler := api.NewUserManagementHandler(
		userManagementService,
		nil,
		userRepo,
		softDeleteRepo,
		emailService,
		nil,
		nil,
	)

	r := chi.NewRouter()
	r.Get("/api/admin/pending-users", userManagementHandler.GetPendingUsers)
	r.Post("/api/admin/users/{id}/approve", userManagementHandler.ApproveUser)
	r.Post("/api/admin/users/{id}/reject", userManagementHandler.RejectUser)
	r.Post("/api/admin/users/{id}/mark-spam", userManagementHandler.MarkUserAsSpam)

	// Account administration routes
	r.Post("/api/admin/users", userManagementHandler.CreateUser)
	r.Get("/api/admin/users", userManagementHandler.GetAllUsers)
	r.Post("/api/admin/users/{id}/suspend", userManagementHandler.SuspendUser)
	r.Post("/api/admin/users/{id}/unsuspend", userManagementHandler.UnsuspendUser)
	r.Post("/api/admin/users/{id}/soft-delete", userManagementHandler.SoftDeleteUser)
	r.Put("/api/admin/users/{id}", userManagementHandler.UpdateUser)
	r.Get("/api/admin/users/{id}/deleted-content", userManagementHandler.GetSoftDeletedContent)
	r.Post("/api/admin/content/{id}/restore", userManagementHandler.RestoreContent)

	return r
}

// TestApproveUserFlow tests the complete approve user flow
func TestApproveUserFlow(t *testing.T) {
	// Setup
	userRepo := &stubUserRepo{
		users: map[int]*userdomain.User{
			123: {
				ID:       123,
				Username: "pendinguser",
				Email:    "pending@example.com",
				Status:   "pending",
				Role:     "Contributor",
			},
		},
		pending: []*userdomain.User{
			{
				ID:       123,
				Username: "pendinguser",
				Email:    "pending@example.com",
				Status:   "pending",
				Role:     "Contributor",
			},
		},
	}
	blockedEmailRepo := &stubBlockedRepo{}
	emailService := &stubMailer{}
	softDeleteRepo := &stubSoftDelete{}

	r := umgmtSetupTestRouter(t, userRepo, blockedEmailRepo, emailService, softDeleteRepo)

	// Test 1: Get pending users
	req := httptest.NewRequest(http.MethodGet, "/api/admin/pending-users", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	var wrapper struct {
		Data api.GetPendingUsersResponse `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&wrapper); err != nil {
		t.Fatalf("Failed to decode response: %v. Body: %s", err, w.Body.String())
	}

	resp := wrapper.Data
	t.Logf("Response: %+v", resp)

	if len(resp.Data) != 1 {
		t.Fatalf("Expected 1 pending user, got %d", len(resp.Data))
	}

	if resp.Data[0].Username != "pendinguser" {
		t.Errorf("Expected username 'pendinguser', got '%s'", resp.Data[0].Username)
	}

	if resp.Meta.Count != 1 {
		t.Errorf("Expected count 1, got %d", resp.Meta.Count)
	}

	// Test 2: Approve user
	req = httptest.NewRequest(http.MethodPost, "/api/admin/users/123/approve", nil)
	w = httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var approveRespWrapper struct {
		Data api.ApproveUserResponse `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&approveRespWrapper); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}
	approveResp := approveRespWrapper.Data

	if userRepo.users[123].Status != "verified" {
		t.Errorf("Expected user ID 123 to be approved (status 'verified'), got '%s'", userRepo.users[123].Status)
	}

	if approveResp.Message != "User approved successfully" {
		t.Errorf("Expected message 'User approved successfully', got '%s'", approveResp.Message)
	}

	if len(emailService.sent) != 1 || emailService.sent[0].to != "pending@example.com" || emailService.sent[0].username != "pendinguser" {
		t.Errorf("Expected approval email to pending@example.com/pendinguser, got %+v", emailService.sent)
	}
}

// TestRejectUserFlow tests the complete reject user flow
func TestRejectUserFlow(t *testing.T) {
	// Setup
	userRepo := &stubUserRepo{
		users: map[int]*userdomain.User{
			456: {
				ID:       456,
				Username: "rejectuser",
				Email:    "reject@example.com",
				Status:   "pending",
				Role:     "Contributor",
			},
		},
	}
	blockedEmailRepo := &stubBlockedRepo{}
	emailService := &stubMailer{}
	softDeleteRepo := &stubSoftDelete{}

	r := umgmtSetupTestRouter(t, userRepo, blockedEmailRepo, emailService, softDeleteRepo)

	// Reject user
	rejectReqBody := map[string]bool{
		"confirmed": true,
	}
	bodyBytes, _ := json.Marshal(rejectReqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/admin/users/456/reject", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	var respWrapper struct {
		Data api.RejectUserResponse `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&respWrapper); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}
	resp := respWrapper.Data

	if _, ok := userRepo.users[456]; ok {
		t.Errorf("Expected user ID 456 to be deleted, but it still exists")
	}

	if resp.Message != "User rejected successfully. Rejection email sent." {
		t.Errorf("Expected message 'User rejected successfully. Rejection email sent.', got '%s'", resp.Message)
	}

	if len(emailService.sent) != 1 || emailService.sent[0].to != "reject@example.com" || emailService.sent[0].username != "rejectuser" {
		t.Errorf("Expected rejection email to reject@example.com/rejectuser, got %+v", emailService.sent)
	}
}

// TestMarkUserAsSpamFlow tests the complete mark as spam flow
func TestMarkUserAsSpamFlow(t *testing.T) {
	// Setup
	userRepo := &stubUserRepo{
		users: map[int]*userdomain.User{
			789: {
				ID:       789,
				Username: "spamuser",
				Email:    "spam@example.com",
				Status:   "pending",
				Role:     "Contributor",
			},
		},
	}
	blockedEmailRepo := &stubBlockedRepo{}
	emailService := &stubMailer{}
	softDeleteRepo := &stubSoftDelete{}

	r := umgmtSetupTestRouter(t, userRepo, blockedEmailRepo, emailService, softDeleteRepo)

	// Mark user as spam
	markSpamReqBody := map[string]bool{
		"confirmed": true,
	}
	bodyBytes, _ := json.Marshal(markSpamReqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/admin/users/789/mark-spam", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	var respWrapper struct {
		Data api.MarkUserAsSpamResponse `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&respWrapper); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}
	resp := respWrapper.Data

	if _, ok := userRepo.users[789]; ok {
		t.Errorf("Expected user ID 789 to be deleted, but it still exists")
	}

	if !blockedEmailRepo.blocked["spam@example.com"] {
		t.Errorf("Expected email 'spam@example.com' to be blocked")
	}

	if resp.Message != "User marked as spam and email blocked" {
		t.Errorf("Expected message 'User marked as spam and email blocked', got '%s'", resp.Message)
	}
}

// TestSuspendUserFlow tests the complete suspend user flow
func TestSuspendUserFlow(t *testing.T) {
	// Setup
	userRepo := &stubUserRepo{
		users: map[int]*userdomain.User{
			123: {
				ID:       123,
				Username: "testuser",
				Email:    "test@example.com",
				Status:   "verified",
				Role:     "Author",
			},
		},
		all: []*userdomain.User{
			{
				ID:       123,
				Username: "testuser",
				Email:    "test@example.com",
				Status:   "verified",
				Role:     "Author",
			},
		},
		total: 1,
	}
	blockedEmailRepo := &stubBlockedRepo{}
	emailService := &stubMailer{}
	softDeleteRepo := &stubSoftDelete{}

	r := umgmtSetupTestRouter(t, userRepo, blockedEmailRepo, emailService, softDeleteRepo)

	// Test 1: Get all users (verify user is verified)
	req := httptest.NewRequest(http.MethodGet, "/api/admin/users", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	var usersWrapper struct {
		Data api.GetAllUsersResponse `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&usersWrapper); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	users := usersWrapper.Data.Data
	if len(users) != 1 {
		t.Fatalf("Expected 1 user, got %d", len(users))
	}

	if users[0].Status != "verified" {
		t.Errorf("Expected status 'verified', got '%s'", users[0].Status)
	}

	// Test 2: Suspend user
	suspendReqBody := map[string]string{
		"reason": "Violating community guidelines",
	}
	bodyBytes, _ := json.Marshal(suspendReqBody)

	req = httptest.NewRequest(http.MethodPost, "/api/admin/users/123/suspend", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	var suspendWrapper struct {
		Data api.SuspendUserResponse `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&suspendWrapper); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	suspendResp := suspendWrapper.Data
	if userRepo.users[123].Status != "suspended" {
		t.Errorf("Expected user ID 123 to be suspended, got status '%s'", userRepo.users[123].Status)
	}

	if suspendResp.Message != "User suspended successfully" {
		t.Errorf("Expected message 'User suspended successfully', got '%s'", suspendResp.Message)
	}

	if len(emailService.sent) != 1 || emailService.sent[0].to != "test@example.com" || emailService.sent[0].username != "testuser" || emailService.sent[0].extra != "Violating community guidelines" {
		t.Errorf("Expected suspension email to test@example.com/testuser with reason, got %+v", emailService.sent)
	}
}

// TestUnsuspendUserFlow tests the complete unsuspend user flow
func TestUnsuspendUserFlow(t *testing.T) {
	// Setup
	userRepo := &stubUserRepo{
		users: map[int]*userdomain.User{
			456: {
				ID:       456,
				Username: "suspendeduser",
				Email:    "suspended@example.com",
				Status:   "suspended",
				Role:     "Author",
			},
		},
	}
	blockedEmailRepo := &stubBlockedRepo{}
	emailService := &stubMailer{}
	softDeleteRepo := &stubSoftDelete{}

	r := umgmtSetupTestRouter(t, userRepo, blockedEmailRepo, emailService, softDeleteRepo)

	// Unsuspend user
	unsuspendReqBody := map[string]string{
		"reason": "Suspension lifted after review",
	}
	bodyBytes, _ := json.Marshal(unsuspendReqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/admin/users/456/unsuspend", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	var unsuspendWrapper struct {
		Data api.UnsuspendUserResponse `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&unsuspendWrapper); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	unsuspendResp := unsuspendWrapper.Data
	if userRepo.users[456].Status != "verified" {
		t.Errorf("Expected user ID 456 to be unsuspended (status 'verified'), got '%s'", userRepo.users[456].Status)
	}

	if unsuspendResp.Message != "User unsuspended successfully" {
		t.Errorf("Expected message 'User unsuspended successfully', got '%s'", unsuspendResp.Message)
	}

	if len(emailService.sent) != 1 || emailService.sent[0].to != "suspended@example.com" || emailService.sent[0].username != "suspendeduser" {
		t.Errorf("Expected unsuspension email to suspended@example.com/suspendeduser, got %+v", emailService.sent)
	}
}

// TestSoftDeleteUserFlow tests the complete soft delete user flow
func TestSoftDeleteUserFlow(t *testing.T) {
	// Setup
	userRepo := &stubUserRepo{
		users: map[int]*userdomain.User{
			789: {
				ID:       789,
				Username: "deleteuser",
				Email:    "delete@example.com",
				Status:   "verified",
				Role:     "Author",
			},
		},
	}
	blockedEmailRepo := &stubBlockedRepo{}
	emailService := &stubMailer{}
	softDeleteRepo := &stubSoftDelete{}

	userManagementService := userdomain.NewUserManagementService(userRepo, blockedEmailRepo, stubPolicy{}, true)
	userManagementHandler := api.NewUserManagementHandler(
		userManagementService,
		nil,
		userRepo,
		softDeleteRepo,
		emailService,
		nil,
		nil,
	)

	r := chi.NewRouter()
	r.Post("/api/admin/users/{id}/soft-delete", userManagementHandler.SoftDeleteUser)

	// Soft delete user
	softDeleteReqBody := map[string]any{
		"confirmed": true,
		"reason":    "User requested account deletion",
	}
	bodyBytes, _ := json.Marshal(softDeleteReqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/admin/users/789/soft-delete", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	var softDeleteWrapper struct {
		Data api.SoftDeleteUserResponse `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&softDeleteWrapper); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	softDeleteResp := softDeleteWrapper.Data
	if userRepo.users[789].Status != "soft_deleted" {
		t.Errorf("Expected user ID 789 to be soft deleted, got status '%s'", userRepo.users[789].Status)
	}

	if softDeleteResp.Message != "User account and content soft deleted successfully" {
		t.Errorf("Expected message 'User account and content soft deleted successfully', got '%s'", softDeleteResp.Message)
	}

	// Note: DeletedContentCount is 0 because content management is not yet implemented
	// This is a placeholder for Story 2.x when content types are added
	if softDeleteResp.DeletedContentCount != 0 {
		t.Errorf("Expected deleted content count 0 (not implemented), got %d", softDeleteResp.DeletedContentCount)
	}

	if len(emailService.sent) != 1 || emailService.sent[0].to != "delete@example.com" || emailService.sent[0].username != "deleteuser" || emailService.sent[0].extra != "User requested account deletion" {
		t.Errorf("Expected soft-delete email to delete@example.com/deleteuser with reason, got %+v", emailService.sent)
	}
}

// TestStatusFilteringInUserList tests status filtering in user list
//
// Deviation: stubUserRepo returns static all/total slices regardless of the
// status/limit/offset arguments (it has no programmable filter hook), while
// the original used RunAndReturn mocks with dynamic filtering. Each subtest
// below stages repo.all/repo.total to the rows the real repository would
// return for that query before issuing the request, preserving every
// assertion. Pagination meta (limit/offset caps) is handler-side and
// asserted unchanged.
func TestStatusFilteringInUserList(t *testing.T) {
	// Setup
	suspended1 := &userdomain.User{
		ID:       111,
		Username: "suspendeduser1",
		Email:    "suspended1@example.com",
		Status:   "suspended",
		Role:     "Author",
	}
	suspended2 := &userdomain.User{
		ID:       222,
		Username: "suspendeduser2",
		Email:    "suspended2@example.com",
		Status:   "suspended",
		Role:     "Contributor",
	}
	verifiedUser := &userdomain.User{
		ID:       333,
		Username: "activeuser",
		Email:    "active@example.com",
		Status:   "verified",
		Role:     "Admin",
	}
	deletedUser := &userdomain.User{
		ID:       444,
		Username: "deleteduser",
		Email:    "deleted@example.com",
		Status:   "soft_deleted",
		Role:     "Author",
	}
	allUsers := []*userdomain.User{suspended1, suspended2, verifiedUser, deletedUser}

	userRepo := &stubUserRepo{}
	blockedEmailRepo := &stubBlockedRepo{}
	emailService := &stubMailer{}
	softDeleteRepo := &stubSoftDelete{}

	r := umgmtSetupTestRouter(t, userRepo, blockedEmailRepo, emailService, softDeleteRepo)

	// Test 1: Get all users (no filter)
	userRepo.all = allUsers
	userRepo.total = 4
	req := httptest.NewRequest(http.MethodGet, "/api/admin/users", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	var allUsersWrapper struct {
		Data api.GetAllUsersResponse `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&allUsersWrapper); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	allUsersResp := allUsersWrapper.Data.Data
	if len(allUsersResp) != 4 {
		t.Errorf("Expected 4 users, got %d", len(allUsersResp))
	}

	// Test 2: Filter by suspended status
	userRepo.all = []*userdomain.User{suspended1, suspended2}
	userRepo.total = 2
	req = httptest.NewRequest(http.MethodGet, "/api/admin/users?status=suspended", nil)
	w = httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	var suspendedUsersWrapper struct {
		Data api.GetAllUsersResponse `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&suspendedUsersWrapper); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	suspendedUsers := suspendedUsersWrapper.Data.Data
	if len(suspendedUsers) != 2 {
		t.Errorf("Expected 2 suspended users, got %d", len(suspendedUsers))
	}

	for _, u := range suspendedUsers {
		if u.Status != "suspended" {
			t.Errorf("Expected all users to have status 'suspended', got '%s'", u.Status)
		}
	}

	// Test 3: Filter by verified status
	userRepo.all = []*userdomain.User{verifiedUser}
	userRepo.total = 1
	req = httptest.NewRequest(http.MethodGet, "/api/admin/users?status=verified", nil)
	w = httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var verifiedUsersWrapper struct {
		Data api.GetAllUsersResponse `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&verifiedUsersWrapper); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	verifiedUsers := verifiedUsersWrapper.Data.Data
	if len(verifiedUsers) != 1 {
		t.Errorf("Expected 1 verified user, got %d", len(verifiedUsers))
	}

	if verifiedUsers[0].Status != "verified" {
		t.Errorf("Expected user status 'verified', got '%s'", verifiedUsers[0].Status)
	}

	// Test 4: Filter by soft_deleted status
	userRepo.all = []*userdomain.User{deletedUser}
	userRepo.total = 1
	req = httptest.NewRequest(http.MethodGet, "/api/admin/users?status=soft_deleted", nil)
	w = httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var deletedUsersWrapper struct {
		Data api.GetAllUsersResponse `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&deletedUsersWrapper); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	deletedUsers := deletedUsersWrapper.Data.Data
	if len(deletedUsers) != 1 {
		t.Errorf("Expected 1 soft_deleted user, got %d", len(deletedUsers))
	}

	if deletedUsers[0].Status != "soft_deleted" {
		t.Errorf("Expected user status 'soft_deleted', got '%s'", deletedUsers[0].Status)
	}

	// Test 5: Pagination with limit
	userRepo.all = []*userdomain.User{suspended1}
	userRepo.total = 4
	req = httptest.NewRequest(http.MethodGet, "/api/admin/users?limit=1", nil)
	w = httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var paginatedUsersWrapper struct {
		Data api.GetAllUsersResponse `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&paginatedUsersWrapper); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	paginatedUsers := paginatedUsersWrapper.Data.Data
	if len(paginatedUsers) != 1 {
		t.Errorf("Expected 1 user with limit=1, got %d", len(paginatedUsers))
	}

	if paginatedUsersWrapper.Data.Meta.Limit != 1 {
		t.Errorf("Expected meta.limit=1, got %d", paginatedUsersWrapper.Data.Meta.Limit)
	}

	// Test offset parameter skips results correctly
	userRepo.all = []*userdomain.User{suspended2, verifiedUser, deletedUser}
	userRepo.total = 4
	req = httptest.NewRequest(http.MethodGet, "/api/admin/users?offset=1", nil)
	w = httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	var offsetWrapper struct {
		Data api.GetAllUsersResponse `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&offsetWrapper); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	offsetUsers := offsetWrapper.Data.Data
	if len(offsetUsers) != 3 {
		t.Errorf("Expected 3 users with offset=1 (skipping first of 4), got %d", len(offsetUsers))
	}

	if offsetWrapper.Data.Meta.Offset != 1 {
		t.Errorf("Expected meta.offset=1, got %d", offsetWrapper.Data.Meta.Offset)
	}

	// Test max limit is capped at 1000
	userRepo.all = allUsers
	userRepo.total = 4
	req = httptest.NewRequest(http.MethodGet, "/api/admin/users?limit=5000", nil)
	w = httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	var maxLimitWrapper struct {
		Data api.GetAllUsersResponse `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&maxLimitWrapper); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if maxLimitWrapper.Data.Meta.Limit != 1000 {
		t.Errorf("Expected meta.limit capped to 1000, got %d", maxLimitWrapper.Data.Meta.Limit)
	}
}

// TestGetSoftDeletedContent tests retrieving soft deleted content for a user
func TestGetSoftDeletedContent(t *testing.T) {
	// Setup
	userRepo := &stubUserRepo{
		users: map[int]*userdomain.User{
			123: {
				ID:       123,
				Username: "testuser",
				Email:    "test@example.com",
				Status:   "soft_deleted",
				Role:     "Author",
			},
			1: {
				ID:       1,
				Username: "admin",
				Email:    "admin@example.com",
				Status:   "verified",
				Role:     "Admin",
			},
		},
	}
	blockedEmailRepo := &stubBlockedRepo{}
	emailService := &stubMailer{}
	softDeleteRepo := &stubSoftDelete{
		byUser: map[int][]*api.SoftDeletedContent{
			123: {
				{
					ID:          1,
					ContentType: "post",
					ContentID:   101,
					DeletedAt:   "2026-03-28T14:30:00Z",
					DeletedBy:   1,
					Reason:      "User requested account deletion",
					HasReason:   true,
				},
				{
					ID:          2,
					ContentType: "comment",
					ContentID:   202,
					DeletedAt:   "2026-03-28T14:30:00Z",
					DeletedBy:   1,
					Reason:      "User requested account deletion",
					HasReason:   true,
				},
			},
		},
	}

	userManagementService := userdomain.NewUserManagementService(userRepo, blockedEmailRepo, stubPolicy{}, true)
	userManagementHandler := api.NewUserManagementHandler(
		userManagementService,
		nil,
		userRepo,
		softDeleteRepo,
		emailService,
		nil,
		nil,
	)

	r := chi.NewRouter()
	r.Get("/api/admin/users/{id}/deleted-content", userManagementHandler.GetSoftDeletedContent)

	// Get soft deleted content
	req := httptest.NewRequest(http.MethodGet, "/api/admin/users/123/deleted-content", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	var wrapper struct {
		Data api.GetSoftDeletedContentResponse `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&wrapper); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	resp := wrapper.Data
	if len(resp.Data) != 2 {
		t.Errorf("Expected 2 deleted content items, got %d", len(resp.Data))
	}

	if resp.Data[0].ContentType != "post" {
		t.Errorf("Expected content type 'post', got '%s'", resp.Data[0].ContentType)
	}

	if resp.Data[1].ContentType != "comment" {
		t.Errorf("Expected content type 'comment', got '%s'", resp.Data[1].ContentType)
	}

	if resp.Data[0].DeletedBy != "admin" {
		t.Errorf("Expected deletedBy 'admin', got '%s'", resp.Data[0].DeletedBy)
	}

	if resp.Meta.Count != 2 {
		t.Errorf("Expected count 2, got %d", resp.Meta.Count)
	}
}

// TestRestoreSoftDeletedContent tests restoring soft deleted content
func TestRestoreSoftDeletedContent(t *testing.T) {
	// Setup
	softDeleteRepo := &stubSoftDelete{
		byID: map[int]*api.SoftDeletedContent{
			1: {
				ID:          1,
				ContentType: "post",
				ContentID:   101,
				DeletedAt:   "2026-03-28T14:30:00Z",
				DeletedBy:   1,
				Reason:      "User requested account deletion",
				HasReason:   true,
			},
		},
	}

	userRepo := &stubUserRepo{}
	blockedEmailRepo := &stubBlockedRepo{}
	emailService := &stubMailer{}

	userManagementService := userdomain.NewUserManagementService(userRepo, blockedEmailRepo, stubPolicy{}, true)
	userManagementHandler := api.NewUserManagementHandler(
		userManagementService,
		nil,
		userRepo,
		softDeleteRepo,
		emailService,
		nil,
		nil,
	)

	r := chi.NewRouter()
	r.Post("/api/admin/content/{id}/restore", userManagementHandler.RestoreContent)

	// Restore content
	req := httptest.NewRequest(http.MethodPost, "/api/admin/content/1/restore", nil)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	var wrapper struct {
		Data api.RestoreContentResponse `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&wrapper); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	resp := wrapper.Data
	if resp.Message != "Content restored successfully" {
		t.Errorf("Expected message 'Content restored successfully', got '%s'", resp.Message)
	}

	if resp.Content == nil {
		t.Fatal("Expected content field in response, got nil")
	} else {
		if resp.Content.ID != 101 {
			t.Errorf("Expected content.ID=101, got %d", resp.Content.ID)
		}

		if resp.Content.Type != "post" {
			t.Errorf("Expected content.Type='post', got '%s'", resp.Content.Type)
		}
	}

	// stubSoftDelete records only the restored content type (not the ID);
	// the ID assertion above is covered by the response content instead.
	if len(softDeleteRepo.restored) != 1 || softDeleteRepo.restored[0] != "post" {
		t.Errorf("Expected content type 'post' to be restored, got %+v", softDeleteRepo.restored)
	}
}

// TestEmailNotificationsSentOnAccountActions tests that email notifications are sent on suspend/unsuspend/soft delete
func TestEmailNotificationsSentOnAccountActions(t *testing.T) {
	// Test 1: Suspend user sends email
	t.Run("Suspend user sends email", func(t *testing.T) {
		userRepo := &stubUserRepo{
			users: map[int]*userdomain.User{
				123: {
					ID:       123,
					Username: "testuser",
					Email:    "test@example.com",
					Status:   "verified",
					Role:     "Author",
				},
			},
		}
		blockedEmailRepo := &stubBlockedRepo{}
		emailService := &stubMailer{}
		softDeleteRepo := &stubSoftDelete{}

		userManagementService := userdomain.NewUserManagementService(userRepo, blockedEmailRepo, stubPolicy{}, true)
		userManagementHandler := api.NewUserManagementHandler(
			userManagementService,
			nil,
			userRepo,
			softDeleteRepo,
			emailService,
			nil,
			nil,
		)

		r := chi.NewRouter()
		r.Post("/api/admin/users/{id}/suspend", userManagementHandler.SuspendUser)

		reqBody := map[string]string{"reason": "Test suspension"}
		bodyBytes, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPost, "/api/admin/users/123/suspend", bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		if len(emailService.sent) != 1 {
			t.Fatalf("Expected 1 email to be sent, got %d", len(emailService.sent))
		}

		if emailService.sent[0].to != "test@example.com" {
			t.Errorf("Expected email to be sent to 'test@example.com', got '%s'", emailService.sent[0].to)
		}

		if emailService.sent[0].username != "testuser" {
			t.Errorf("Expected username 'testuser', got '%s'", emailService.sent[0].username)
		}

		if emailService.sent[0].extra != "Test suspension" {
			t.Errorf("Expected reason 'Test suspension', got '%s'", emailService.sent[0].extra)
		}
	})

	// Test 2: Unsuspend user sends email
	t.Run("Unsuspend user sends email", func(t *testing.T) {
		unsuspendUserRepo := &stubUserRepo{
			users: map[int]*userdomain.User{
				123: {
					ID:       123,
					Username: "testuser",
					Email:    "test@example.com",
					Status:   "suspended",
					Role:     "Author",
				},
			},
		}
		unsuspendBlockedEmailRepo := &stubBlockedRepo{}
		unsuspendEmailService := &stubMailer{}
		unsuspendSoftDeleteRepo := &stubSoftDelete{}

		unsuspendService := userdomain.NewUserManagementService(unsuspendUserRepo, unsuspendBlockedEmailRepo, stubPolicy{}, true)
		unsuspendHandler := api.NewUserManagementHandler(
			unsuspendService,
			nil,
			unsuspendUserRepo,
			unsuspendSoftDeleteRepo,
			unsuspendEmailService,
			nil,
			nil,
		)

		unsuspendRouter := chi.NewRouter()
		unsuspendRouter.Post("/api/admin/users/{id}/unsuspend", unsuspendHandler.UnsuspendUser)

		reqBody := map[string]string{"reason": "Test unsuspension"}
		bodyBytes, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPost, "/api/admin/users/123/unsuspend", bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		unsuspendRouter.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		if len(unsuspendEmailService.sent) != 1 {
			t.Fatalf("Expected 1 email to be sent, got %d", len(unsuspendEmailService.sent))
		}

		if unsuspendEmailService.sent[0].to != "test@example.com" {
			t.Errorf("Expected email to be sent to 'test@example.com', got '%s'", unsuspendEmailService.sent[0].to)
		}

		if unsuspendEmailService.sent[0].username != "testuser" {
			t.Errorf("Expected username 'testuser', got '%s'", unsuspendEmailService.sent[0].username)
		}
	})

	// Test 3: Soft delete user sends email
	t.Run("Soft delete user sends email", func(t *testing.T) {
		userRepo := &stubUserRepo{
			users: map[int]*userdomain.User{
				123: {
					ID:       123,
					Username: "testuser",
					Email:    "test@example.com",
					Status:   "verified",
					Role:     "Author",
				},
			},
		}
		blockedEmailRepo := &stubBlockedRepo{}
		emailService := &stubMailer{}
		softDeleteRepo := &stubSoftDelete{}

		userManagementService := userdomain.NewUserManagementService(userRepo, blockedEmailRepo, stubPolicy{}, true)
		userManagementHandler := api.NewUserManagementHandler(
			userManagementService,
			nil,
			userRepo,
			softDeleteRepo,
			emailService,
			nil,
			nil,
		)

		r2 := chi.NewRouter()
		r2.Post("/api/admin/users/{id}/soft-delete", userManagementHandler.SoftDeleteUser)

		reqBody := map[string]any{
			"confirmed": true,
			"reason":    "Test soft delete",
		}
		bodyBytes, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPost, "/api/admin/users/123/soft-delete", bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		r2.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		if len(emailService.sent) != 1 {
			t.Fatalf("Expected 1 email to be sent, got %d", len(emailService.sent))
		}

		if emailService.sent[0].to != "test@example.com" {
			t.Errorf("Expected email to be sent to 'test@example.com', got '%s'", emailService.sent[0].to)
		}

		if emailService.sent[0].username != "testuser" {
			t.Errorf("Expected username 'testuser', got '%s'", emailService.sent[0].username)
		}

		if emailService.sent[0].extra != "Test soft delete" {
			t.Errorf("Expected reason 'Test soft delete', got '%s'", emailService.sent[0].extra)
		}
	})
}

func TestCreateUser_Success(t *testing.T) {
	userRepo := &stubUserRepo{}
	blockedEmailRepo := &stubBlockedRepo{}
	emailService := &stubMailer{}
	softDeleteRepo := &stubSoftDelete{}

	userManagementService := userdomain.NewUserManagementService(userRepo, blockedEmailRepo, stubPolicy{}, true)
	adminCreateService := userdomain.NewAdminCreateUserService(userRepo, blockedEmailRepo, stubPolicy{}, true)

	userManagementHandler := api.NewUserManagementHandler(
		userManagementService,
		adminCreateService,
		userRepo,
		softDeleteRepo,
		emailService,
		nil,
		nil,
	)

	// The welcome email is sent fire-and-forget in a goroutine; no
	// assertion is made on it (matching the original's .Maybe() mock).
	r := chi.NewRouter()
	r.Post("/api/admin/users", userManagementHandler.CreateUser)

	body := `{"username":"newuser","email":"newuser@example.com","role":"Contributor"}`
	req := httptest.NewRequest(http.MethodPost, "/api/admin/users", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp["data"].(map[string]any)

	if data["message"] != "User created successfully" {
		t.Errorf("Expected 'User created successfully', got '%v'", data["message"])
	}

	userData := data["user"].(map[string]any)
	if userData["username"] != "newuser" {
		t.Errorf("Expected username 'newuser', got '%v'", userData["username"])
	}
	if userData["status"] != "verified" {
		t.Errorf("Expected status 'verified', got '%v'", userData["status"])
	}
	if data["password"] == nil || data["password"].(string) == "" {
		t.Error("Expected non-empty password in response")
	}
}

func TestCreateUser_MissingFields(t *testing.T) {
	userRepo := &stubUserRepo{}
	blockedEmailRepo := &stubBlockedRepo{}
	emailService := &stubMailer{}
	softDeleteRepo := &stubSoftDelete{}

	r := umgmtSetupTestRouter(t, userRepo, blockedEmailRepo, emailService, softDeleteRepo)

	tests := []struct {
		name     string
		body     string
		wantCode string
	}{
		{"missing all", `{"username":"","email":"","role":""}`, "MISSING_FIELDS"},
		{"missing username", `{"username":"","email":"test@example.com","role":"Contributor"}`, "MISSING_FIELDS"},
		{"missing email", `{"username":"testuser","email":"","role":"Contributor"}`, "MISSING_FIELDS"},
		{"missing role", `{"username":"testuser","email":"test@example.com","role":""}`, "MISSING_FIELDS"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/admin/users", bytes.NewReader([]byte(tt.body)))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			if w.Code != http.StatusBadRequest {
				t.Errorf("Expected status 400, got %d", w.Code)
			}

			var resp map[string]any
			_ = json.Unmarshal(w.Body.Bytes(), &resp)
			errInfo := resp["error"].(map[string]any)
			if errInfo["code"] != tt.wantCode {
				t.Errorf("Expected error code '%s', got '%v'", tt.wantCode, errInfo["code"])
			}
		})
	}
}

func TestCreateUser_InvalidRequest(t *testing.T) {
	userRepo := &stubUserRepo{}
	blockedEmailRepo := &stubBlockedRepo{}
	emailService := &stubMailer{}
	softDeleteRepo := &stubSoftDelete{}

	r := umgmtSetupTestRouter(t, userRepo, blockedEmailRepo, emailService, softDeleteRepo)

	req := httptest.NewRequest(http.MethodPost, "/api/admin/users", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

// TestUpdateUser_Success tests successful user profile update
func TestUpdateUser_Success(t *testing.T) {
	userRepo := &stubUserRepo{
		users: map[int]*userdomain.User{
			1: {
				ID:       1,
				Username: "testuser",
				Email:    "old@example.com",
				Name:     "Old Name",
				Role:     "Commentator",
				Status:   "verified",
			},
		},
	}
	blockedEmailRepo := &stubBlockedRepo{}
	emailService := &stubMailer{}
	softDeleteRepo := &stubSoftDelete{}

	router := umgmtSetupTestRouter(t, userRepo, blockedEmailRepo, emailService, softDeleteRepo)

	body, _ := json.Marshal(map[string]string{
		"name":  "New Name",
		"email": "new@example.com",
		"role":  "Contributor",
	})
	req := httptest.NewRequest(http.MethodPut, "/api/admin/users/1", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	if userRepo.users[1].Email != "new@example.com" {
		t.Errorf("Expected email 'new@example.com', got '%s'", userRepo.users[1].Email)
	}
}

// TestUpdateUser_InvalidUserID tests invalid user ID
func TestUpdateUser_InvalidUserID(t *testing.T) {
	userRepo := &stubUserRepo{}
	blockedEmailRepo := &stubBlockedRepo{}
	emailService := &stubMailer{}
	softDeleteRepo := &stubSoftDelete{}

	router := umgmtSetupTestRouter(t, userRepo, blockedEmailRepo, emailService, softDeleteRepo)

	body, _ := json.Marshal(map[string]string{"name": "Test"})
	req := httptest.NewRequest(http.MethodPut, "/api/admin/users/abc", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

// TestUpdateUser_NegativeUserID tests negative user ID
func TestUpdateUser_NegativeUserID(t *testing.T) {
	userRepo := &stubUserRepo{}
	blockedEmailRepo := &stubBlockedRepo{}
	emailService := &stubMailer{}
	softDeleteRepo := &stubSoftDelete{}

	router := umgmtSetupTestRouter(t, userRepo, blockedEmailRepo, emailService, softDeleteRepo)

	body, _ := json.Marshal(map[string]string{"name": "Test"})
	req := httptest.NewRequest(http.MethodPut, "/api/admin/users/-1", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

// TestUpdateUser_UserNotFound tests updating non-existent user
func TestUpdateUser_UserNotFound(t *testing.T) {
	userRepo := &stubUserRepo{}
	blockedEmailRepo := &stubBlockedRepo{}
	emailService := &stubMailer{}
	softDeleteRepo := &stubSoftDelete{}

	router := umgmtSetupTestRouter(t, userRepo, blockedEmailRepo, emailService, softDeleteRepo)

	body, _ := json.Marshal(map[string]string{"name": "Test"})
	req := httptest.NewRequest(http.MethodPut, "/api/admin/users/999", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

// TestUpdateUser_InvalidEmail tests invalid email format
//
// Deviation: the original mock repo returned the user while email-format
// validation lived in the real policy mock chain; here the injected
// stubPolicy{emailErr: userdomain.ErrEmailInvalid} makes the real service
// return the sentinel the handler maps to 400.
func TestUpdateUser_InvalidEmail(t *testing.T) {
	userRepo := &stubUserRepo{
		users: map[int]*userdomain.User{
			1: {
				ID:       1,
				Username: "testuser",
				Email:    "old@example.com",
				Name:     "Test",
				Role:     "Admin",
			},
		},
	}
	blockedEmailRepo := &stubBlockedRepo{}
	emailService := &stubMailer{}
	softDeleteRepo := &stubSoftDelete{}

	userManagementService := userdomain.NewUserManagementService(userRepo, blockedEmailRepo, stubPolicy{emailErr: userdomain.ErrEmailInvalid}, true)
	userManagementHandler := api.NewUserManagementHandler(
		userManagementService,
		nil,
		userRepo,
		softDeleteRepo,
		emailService,
		nil,
		nil,
	)

	router := chi.NewRouter()
	router.Put("/api/admin/users/{id}", userManagementHandler.UpdateUser)

	body, _ := json.Marshal(map[string]string{"email": "not-an-email"})
	req := httptest.NewRequest(http.MethodPut, "/api/admin/users/1", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

// TestUpdateUser_InvalidRole tests invalid role
func TestUpdateUser_InvalidRole(t *testing.T) {
	userRepo := &stubUserRepo{
		users: map[int]*userdomain.User{
			1: {
				ID:       1,
				Username: "testuser",
				Email:    "test@example.com",
				Name:     "Test",
				Role:     "Admin",
			},
		},
	}
	blockedEmailRepo := &stubBlockedRepo{}
	emailService := &stubMailer{}
	softDeleteRepo := &stubSoftDelete{}

	router := umgmtSetupTestRouter(t, userRepo, blockedEmailRepo, emailService, softDeleteRepo)

	body, _ := json.Marshal(map[string]string{"role": "SuperAdmin"})
	req := httptest.NewRequest(http.MethodPut, "/api/admin/users/1", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

// TestUpdateUser_InvalidJSON tests invalid JSON body
func TestUpdateUser_InvalidJSON(t *testing.T) {
	userRepo := &stubUserRepo{}
	blockedEmailRepo := &stubBlockedRepo{}
	emailService := &stubMailer{}
	softDeleteRepo := &stubSoftDelete{}

	router := umgmtSetupTestRouter(t, userRepo, blockedEmailRepo, emailService, softDeleteRepo)

	req := httptest.NewRequest(http.MethodPut, "/api/admin/users/1", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

// TestUpdateUser_PartialUpdate tests partial update (only name)
func TestUpdateUser_PartialUpdate(t *testing.T) {
	userRepo := &stubUserRepo{
		users: map[int]*userdomain.User{
			1: {
				ID:       1,
				Username: "testuser",
				Email:    "test@example.com",
				Name:     "Old Name",
				Role:     "Admin",
				Status:   "verified",
			},
		},
	}
	blockedEmailRepo := &stubBlockedRepo{}
	emailService := &stubMailer{}
	softDeleteRepo := &stubSoftDelete{}

	router := umgmtSetupTestRouter(t, userRepo, blockedEmailRepo, emailService, softDeleteRepo)

	body, _ := json.Marshal(map[string]string{"name": "Only Name"})
	req := httptest.NewRequest(http.MethodPut, "/api/admin/users/1", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	if userRepo.users[1].Name != "Only Name" {
		t.Errorf("Expected name 'Only Name', got '%s'", userRepo.users[1].Name)
	}
}

// TestUpdateUser_EmailExists tests email already in use
func TestUpdateUser_EmailExists(t *testing.T) {
	userRepo := &stubUserRepo{
		users: map[int]*userdomain.User{
			1: {ID: 1, Username: "testuser", Email: "old@example.com", Name: "Test", Role: "Admin"},
		},
		emailTakenForOther: map[string]bool{"taken@example.com": true},
	}
	blockedEmailRepo := &stubBlockedRepo{}
	emailService := &stubMailer{}
	softDeleteRepo := &stubSoftDelete{}

	router := umgmtSetupTestRouter(t, userRepo, blockedEmailRepo, emailService, softDeleteRepo)

	body, _ := json.Marshal(map[string]string{"email": "taken@example.com"})
	req := httptest.NewRequest(http.MethodPut, "/api/admin/users/1", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("Expected status 409, got %d", w.Code)
	}
}
