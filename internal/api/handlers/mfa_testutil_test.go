package handlers_test

import (
	"context"
	"time"

	"github.com/aristorinjuang/lesstruct/internal/auth"
	"github.com/aristorinjuang/lesstruct/internal/modules"
	"github.com/aristorinjuang/lesstruct/kernel"
	authapi "github.com/aristorinjuang/lesstruct/modules/auth/api"
	authdomain "github.com/aristorinjuang/lesstruct/modules/auth/domain"
)

// testMFAChallengeRepo is an in-memory MFAChallengeRepo for handler tests. It
// keeps issued challenges keyed by token hash so multi-step flows run without
// mock choreography.
type testMFAChallengeRepo struct {
	challenges map[string]*authdomain.MFAChallenge
	nextID     int
}

func (r *testMFAChallengeRepo) Create(_ context.Context, challenge *authdomain.MFAChallenge) error {
	if r.challenges == nil {
		r.challenges = map[string]*authdomain.MFAChallenge{}
	}
	r.nextID++
	challenge.ID = r.nextID
	r.challenges[challenge.TokenHash] = challenge
	return nil
}

func (r *testMFAChallengeRepo) FindByTokenHash(_ context.Context, tokenHash string) (*authdomain.MFAChallenge, error) {
	return r.challenges[tokenHash], nil
}

func (r *testMFAChallengeRepo) IncrementAttempts(_ context.Context, id, maxAttempts int) (bool, error) {
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

func (r *testMFAChallengeRepo) Consume(_ context.Context, id int) (bool, error) {
	for token, challenge := range r.challenges {
		if challenge.ID == id {
			delete(r.challenges, token)
			return true, nil
		}
	}
	return false, nil
}

func (r *testMFAChallengeRepo) Delete(_ context.Context, id int) error {
	for token, challenge := range r.challenges {
		if challenge.ID == id {
			delete(r.challenges, token)
		}
	}
	return nil
}

func (r *testMFAChallengeRepo) DeleteUserChallenges(_ context.Context, userID int) error {
	for token, challenge := range r.challenges {
		if challenge.UserID == userID {
			delete(r.challenges, token)
		}
	}
	return nil
}

func (r *testMFAChallengeRepo) DeleteExpired(_ context.Context) error {
	for token, challenge := range r.challenges {
		if !time.Now().Before(challenge.ExpiresAt) {
			delete(r.challenges, token)
		}
	}
	return nil
}

// testMFADeliverer captures the emitted login code.
type testMFADeliverer struct {
	to   string
	code string
}

func (d *testMFADeliverer) DeliverMFACode(_ context.Context, to, _ string, code string) error {
	d.to = to
	d.code = code
	return nil
}

// newTestAuthHandler builds the auth handler with an in-memory MFA fixture and
// returns the deliverer holding the emitted login code. The platform JWT
// manager is adapted to the module seam here so callers keep passing the
// manager directly.
func newTestAuthHandler(
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
	opts ...authapi.AuthHandlerOption,
) (*authapi.AuthHandler, *testMFADeliverer) {
	mfaService := authdomain.NewMFAService(
		&testMFAChallengeRepo{},
		authdomain.MFACodeTTL,
		authdomain.MFAResendCooldown,
		authdomain.MFAMaxAttempts,
	)
	deliverer := &testMFADeliverer{}

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
		deliverer,
		blockedEmailRepo,
		opts...,
	), deliverer
}
