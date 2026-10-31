package routes_test

import (
	"context"
	"time"

	"github.com/aristorinjuang/lesstruct/internal/auth"
	"github.com/aristorinjuang/lesstruct/internal/modules"
	"github.com/aristorinjuang/lesstruct/kernel"
	authapi "github.com/aristorinjuang/lesstruct/modules/auth/api"
	authdomain "github.com/aristorinjuang/lesstruct/modules/auth/domain"
)

// routeMFAChallengeRepo is an in-memory MFAChallengeRepo for route tests.
type routeMFAChallengeRepo struct {
	challenges map[string]*authdomain.MFAChallenge
	nextID     int
}

func (r *routeMFAChallengeRepo) Create(_ context.Context, challenge *authdomain.MFAChallenge) error {
	if r.challenges == nil {
		r.challenges = map[string]*authdomain.MFAChallenge{}
	}
	r.nextID++
	challenge.ID = r.nextID
	r.challenges[challenge.TokenHash] = challenge
	return nil
}

func (r *routeMFAChallengeRepo) FindByTokenHash(_ context.Context, tokenHash string) (*authdomain.MFAChallenge, error) {
	return r.challenges[tokenHash], nil
}

func (r *routeMFAChallengeRepo) IncrementAttempts(_ context.Context, id, maxAttempts int) (bool, error) {
	for _, challenge := range r.challenges {
		if challenge.ID == id {
			if challenge.Attempts >= maxAttempts {
				return false, nil
			}
			challenge.Attempts++
			return true, nil
		}
	}
	return false, nil
}

func (r *routeMFAChallengeRepo) Consume(_ context.Context, id int) (bool, error) {
	for token, challenge := range r.challenges {
		if challenge.ID == id {
			delete(r.challenges, token)
			return true, nil
		}
	}
	return false, nil
}

func (r *routeMFAChallengeRepo) Delete(_ context.Context, id int) error {
	for token, challenge := range r.challenges {
		if challenge.ID == id {
			delete(r.challenges, token)
		}
	}
	return nil
}

func (r *routeMFAChallengeRepo) DeleteUserChallenges(_ context.Context, userID int) error {
	for token, challenge := range r.challenges {
		if challenge.UserID == userID {
			delete(r.challenges, token)
		}
	}
	return nil
}

func (r *routeMFAChallengeRepo) DeleteExpired(_ context.Context) error {
	for token, challenge := range r.challenges {
		if !time.Now().Before(challenge.ExpiresAt) {
			delete(r.challenges, token)
		}
	}
	return nil
}

// routeMFADeliverer captures the emitted login code.
type routeMFADeliverer struct {
	code string
}

func (d *routeMFADeliverer) DeliverMFACode(_ context.Context, _, _ string, code string) error {
	d.code = code
	return nil
}

// newRouteAuthHandler builds the auth handler with the MFA fixture the route
// surface requires. The platform JWT manager is adapted to the module seam
// here so callers keep passing the manager directly.
func newRouteAuthHandler(
	authService *authdomain.AuthService,
	jwtManager *auth.JWTManager,
	logger kernel.Logger,
	firstLoginService *authdomain.FirstLoginService,
	registrationService *authdomain.RegistrationService,
	verificationService *authdomain.VerificationService,
	loginService *authdomain.LoginService,
	passwordResetService *authdomain.PasswordResetService,
	userRepo authapi.UserStore,
	failedLoginRepo authdomain.FailedLoginAttemptRepo,
	notificationRepo authapi.NotificationRecorder,
	mailer authapi.Mailer,
	blockedEmailRepo authapi.BlockedEmailChecker,
) *authapi.AuthHandler {
	mfaService := authdomain.NewMFAService(
		&routeMFAChallengeRepo{},
		authdomain.MFACodeTTL,
		authdomain.MFAResendCooldown,
		authdomain.MFAMaxAttempts,
	)

	return authapi.NewAuthHandler(
		authService,
		modules.NewJWTTokenIssuer(jwtManager),
		logger,
		firstLoginService,
		registrationService,
		verificationService,
		loginService,
		mfaService,
		passwordResetService,
		userRepo,
		failedLoginRepo,
		notificationRepo,
		mailer,
		&routeMFADeliverer{},
		blockedEmailRepo,
	)
}
