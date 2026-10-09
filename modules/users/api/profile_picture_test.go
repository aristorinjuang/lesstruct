package api_test

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"testing"

	"github.com/aristorinjuang/lesstruct/modules/users/api"
	userdomain "github.com/aristorinjuang/lesstruct/modules/users/domain"
	"github.com/aristorinjuang/lesstruct/modules/users/domain/profilepicture"
	"github.com/stretchr/testify/require"
)

func createProfilePictureMultipartForm(t *testing.T) (*bytes.Buffer, string) {
	t.Helper()

	img := image.NewRGBA(image.Rect(0, 0, 200, 200))
	for y := range 200 {
		for x := range 200 {
			img.Set(x, y, color.RGBA{uint8(x % 256), uint8(y % 256), 128, 255})
		}
	}

	var imgBuf bytes.Buffer
	err := png.Encode(&imgBuf, img)
	require.NoError(t, err)

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", `form-data; name="image"; filename="avatar.png"`)
	h.Set("Content-Type", "image/png")
	part, err := writer.CreatePart(h)
	require.NoError(t, err)
	_, err = part.Write(imgBuf.Bytes())
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	return &body, writer.FormDataContentType()
}

func TestProfilePictureHandler_DeleteProfilePicture_Success(t *testing.T) {
	storage := &stubPictureStorage{}

	// Pre-save a file
	_, err := storage.Save("testuser_20260101000000.webp", bytes.NewReader([]byte("fake webp")))
	require.NoError(t, err)

	picRepo := &stubPictureRepo{picture: "testuser_20260101000000.webp"}
	userRepo := &stubUserRepo{
		users: map[int]*userdomain.User{
			1: {ID: 1, Username: "testuser", ProfilePicture: "testuser_20260101000000.webp"},
		},
	}
	service := profilepicture.NewService(picRepo, storage, profilepicture.NewProcessor())
	handler := api.NewProfilePictureHandler(service, userRepo, nil)

	req := httptest.NewRequest(http.MethodDelete, "/api/profile/picture", nil)
	req = requestWithPrincipal(req, "1", "testuser")
	w := httptest.NewRecorder()
	handler.DeleteProfilePicture(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data := resp["data"].(map[string]any)
	require.Equal(t, "Profile picture deleted successfully", data["message"])
}

func TestProfilePictureHandler_DeleteProfilePicture_NoPicture(t *testing.T) {
	storage := &stubPictureStorage{}
	processor := profilepicture.NewProcessor()

	picRepo := &stubPictureRepo{picture: ""}
	userRepo := &stubUserRepo{
		users: map[int]*userdomain.User{
			1: {ID: 1, Username: "testuser", ProfilePicture: ""},
		},
	}

	service := profilepicture.NewService(picRepo, storage, processor)
	handler := api.NewProfilePictureHandler(service, userRepo, nil)

	req := httptest.NewRequest(http.MethodDelete, "/api/profile/picture", nil)
	req = requestWithPrincipal(req, "1", "testuser")
	w := httptest.NewRecorder()
	handler.DeleteProfilePicture(w, req)

	require.Equal(t, http.StatusOK, w.Code)
}

func TestProfilePictureHandler_UploadProfilePicture_Success(t *testing.T) {
	storage := &stubPictureStorage{}
	processor := profilepicture.NewProcessor()

	picRepo := &stubPictureRepo{picture: ""}
	userRepo := &stubUserRepo{
		users: map[int]*userdomain.User{
			1: {ID: 1, Username: "testuser", ProfilePicture: ""},
		},
	}

	service := profilepicture.NewService(picRepo, storage, processor)
	handler := api.NewProfilePictureHandler(service, userRepo, nil)

	body, contentType := createProfilePictureMultipartForm(t)
	req := httptest.NewRequest(http.MethodPut, "/api/profile/picture", body)
	req.Header.Set("Content-Type", contentType)
	req = requestWithPrincipal(req, "1", "testuser")
	w := httptest.NewRecorder()
	handler.UploadProfilePicture(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data := resp["data"].(map[string]any)
	require.Contains(t, data["profilePicture"], "testuser_")
}

func TestProfilePictureHandler_AdminUploadUserPicture_Success(t *testing.T) {
	storage := &stubPictureStorage{}
	processor := profilepicture.NewProcessor()

	picRepo := &stubPictureRepo{picture: ""}
	userRepo := &stubUserRepo{
		users: map[int]*userdomain.User{
			5: {ID: 5, Username: "targetuser", ProfilePicture: ""},
		},
	}

	service := profilepicture.NewService(picRepo, storage, processor)
	handler := api.NewProfilePictureHandler(service, userRepo, nil)

	body, contentType := createProfilePictureMultipartForm(t)
	req := httptest.NewRequest(http.MethodPut, "/api/admin/users/5/picture", body)
	req.Header.Set("Content-Type", contentType)
	req = requestWithPrincipal(req, "1", "testuser")
	req.SetPathValue("id", "5")
	w := httptest.NewRecorder()
	handler.AdminUploadUserPicture(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data := resp["data"].(map[string]any)
	require.Contains(t, data["profilePicture"], "targetuser_")
}

func TestProfilePictureHandler_AdminDeleteUserPicture_Success(t *testing.T) {
	storage := &stubPictureStorage{}

	// Pre-save
	_, err := storage.Save("targetuser_20260101000000.webp", bytes.NewReader([]byte("fake webp")))
	require.NoError(t, err)

	picRepo := &stubPictureRepo{picture: "targetuser_20260101000000.webp"}
	userRepo := &stubUserRepo{
		users: map[int]*userdomain.User{
			5: {ID: 5, Username: "targetuser", ProfilePicture: "targetuser_20260101000000.webp"},
		},
	}

	service := profilepicture.NewService(picRepo, storage, profilepicture.NewProcessor())
	handler := api.NewProfilePictureHandler(service, userRepo, nil)

	req := httptest.NewRequest(http.MethodDelete, "/api/admin/users/5/picture", nil)
	req = requestWithPrincipal(req, "1", "testuser")
	req.SetPathValue("id", "5")
	w := httptest.NewRecorder()
	handler.AdminDeleteUserPicture(w, req)

	require.Equal(t, http.StatusOK, w.Code)
}
