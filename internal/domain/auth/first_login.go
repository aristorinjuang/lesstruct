package auth

// FirstLoginService manages first-login setup state by deriving it from the database.
// Setup is considered complete when the admin password differs from the default.
type FirstLoginService struct {
	defaultPasswordHash string
}

// IsSetupComplete returns whether first-login setup is complete by comparing
// the current admin password hash against the default password hash.
func (f *FirstLoginService) IsSetupComplete(currentAdminHash string) bool {
	return currentAdminHash != "" && currentAdminHash != f.defaultPasswordHash
}

// NewFirstLoginService creates a new first-login service
func NewFirstLoginService(defaultPasswordHash string) *FirstLoginService {
	return &FirstLoginService{
		defaultPasswordHash: defaultPasswordHash,
	}
}
