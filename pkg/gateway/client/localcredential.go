package client

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"
	"unicode"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/storage/value"

	types2 "github.com/obot-platform/obot/apiclient/types"
	"github.com/obot-platform/obot/pkg/gateway/types"
	"github.com/obot-platform/obot/pkg/hash"
)

const (
	// LocalAuthProviderName is the synthetic auth-provider name used for local accounts.
	// It follows the upstream naming convention (e.g. "github-auth-provider") to avoid
	// any future collision with a real ToolReference named "local".
	LocalAuthProviderName = "local-auth-provider"

	// LocalAuthProviderNamespace is the namespace used for local account identities.
	LocalAuthProviderNamespace = "default"

	// bcryptCost is the work factor for password hashing.
	// 12 is a reasonable balance between security and login latency (~300 ms on modern hardware).
	bcryptCost = 12

	// minPasswordLength is the minimum required password length.
	minPasswordLength = 8

	// maxPasswordLength caps passwords before bcrypt truncation (72 bytes).
	maxPasswordLength = 72

	// maxFailedAttempts is the number of failures before the account is locked.
	maxFailedAttempts = 5

	// baseLockoutDuration is the initial lockout time; it doubles with each subsequent lockout.
	baseLockoutDuration = time.Minute
)

var (
	localCredGroupResource = schema.GroupResource{
		Group:    "obot.obot.ai",
		Resource: "localcredentials",
	}

	// invalidLocalHashOnce guards lazy initialization of invalidLocalHash.
	// Bcrypt at cost 12 takes ~300 ms; computing it at package init would block startup.
	invalidLocalHashOnce sync.Once
	invalidLocalHash     []byte

	// ErrAccountLocked is returned when a local account is temporarily locked out.
	ErrAccountLocked = errors.New("account temporarily locked")

	// ErrAccountDisabled is returned when a local account has been disabled due to inactivity.
	ErrAccountDisabled = errors.New("account disabled")
)

// getInvalidLocalHash returns a stable bcrypt hash of a placeholder password,
// computed lazily on first use to avoid blocking process startup.
func getInvalidLocalHash() []byte {
	invalidLocalHashOnce.Do(func() {
		invalidLocalHash, _ = bcrypt.GenerateFromPassword([]byte("invalid-placeholder"), bcryptCost)
	})
	return invalidLocalHash
}

// ValidatePasswordComplexity enforces minimum password requirements.
func ValidatePasswordComplexity(plain string) error {
	if len(plain) < minPasswordLength {
		return fmt.Errorf("password must be at least %d characters", minPasswordLength)
	}
	if len(plain) > maxPasswordLength {
		return fmt.Errorf("password must be at most %d characters", maxPasswordLength)
	}

	var hasUpper, hasLower, hasDigit bool
	for _, r := range plain {
		switch {
		case unicode.IsUpper(r):
			hasUpper = true
		case unicode.IsLower(r):
			hasLower = true
		case unicode.IsDigit(r):
			hasDigit = true
		}
	}
	if !hasUpper || !hasLower || !hasDigit {
		return errors.New("password must contain at least one uppercase letter, one lowercase letter, and one digit")
	}
	return nil
}

// CreateLocalCredential hashes the password and stores it for the given user.
// Admin-created accounts are flagged MustChange=true to force a reset on first login.
func (c *Client) CreateLocalCredential(ctx context.Context, userID uint, plainPassword string) error {
	return c.saveLocalCredential(ctx, userID, plainPassword, true)
}

// VerifyLocalCredential looks up the user by username (via the Identity table),
// verifies the password against the stored bcrypt hash, checks account lockout,
// and returns the user along with whether a forced password change is pending.
func (c *Client) VerifyLocalCredential(ctx context.Context, username, plainPassword string) (*types.User, bool, error) {
	authFailed := errors.New("invalid username or password")

	identity, err := c.findLocalIdentity(ctx, username)
	if err != nil {
		return c.handleIdentityNotFound(err, plainPassword, authFailed)
	}

	cred, err := c.getLocalCredential(ctx, identity.UserID)
	if err != nil {
		return c.handleCredentialNotFound(err, plainPassword, authFailed)
	}

	if err := c.checkAccountStatus(cred); err != nil {
		return nil, false, err
	}

	if err := c.decryptLocalCredential(ctx, identity.UserID, cred); err != nil {
		return nil, false, fmt.Errorf("failed to decrypt local credential: %w", err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(cred.PasswordHash), []byte(plainPassword)); err != nil {
		c.recordFailedAttempt(ctx, cred)
		return nil, false, authFailed
	}

	c.recordSuccessfulLogin(ctx, cred)

	user, err := c.UserByID(ctx, FormatUserID(identity.UserID))
	if err != nil {
		return nil, false, err
	}
	return user, cred.MustChange, nil
}

// findLocalIdentity looks up the identity for a local auth user by username.
func (c *Client) findLocalIdentity(ctx context.Context, username string) (*types.Identity, error) {
	var identity types.Identity
	err := c.db.WithContext(ctx).
		Where("auth_provider_name = ? AND auth_provider_namespace = ? AND hashed_provider_user_id = ?",
			LocalAuthProviderName,
			LocalAuthProviderNamespace,
			hash.String(username),
		).First(&identity).Error
	return &identity, err
}

// handleIdentityNotFound handles the case where identity lookup fails.
func (c *Client) handleIdentityNotFound(err error, plainPassword string, authFailed error) (*types.User, bool, error) {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		_ = bcrypt.CompareHashAndPassword(getInvalidLocalHash(), []byte(plainPassword))
		return nil, false, authFailed
	}
	return nil, false, err
}

// handleCredentialNotFound handles the case where credential lookup fails.
func (c *Client) handleCredentialNotFound(err error, plainPassword string, authFailed error) (*types.User, bool, error) {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		_ = bcrypt.CompareHashAndPassword(getInvalidLocalHash(), []byte(plainPassword))
		return nil, false, authFailed
	}
	return nil, false, err
}

// checkAccountStatus verifies the account is not locked or disabled.
func (c *Client) checkAccountStatus(cred *types.LocalCredential) error {
	// Check account lockout before doing any decryption or bcrypt work.
	if !cred.LockedUntil.IsZero() && time.Now().Before(cred.LockedUntil) {
		remaining := time.Until(cred.LockedUntil).Round(time.Second)
		return fmt.Errorf("%w: try again in %s", ErrAccountLocked, remaining)
	}

	// Check if the account has been disabled due to inactivity (Dev-7).
	if cred.Disabled {
		return ErrAccountDisabled
	}
	return nil
}

// recordSuccessfulLogin updates the login timestamp and resets failed attempts.
func (c *Client) recordSuccessfulLogin(ctx context.Context, cred *types.LocalCredential) {
	// Record login time; also reset any failed-attempt counter if needed.
	// DB write error is intentionally ignored (see recordFailedAttempt for rationale).
	now := time.Now()
	updates := map[string]any{"last_login_at": now}
	if cred.FailedAttempts > 0 {
		updates["failed_attempts"] = 0
		updates["locked_until"] = time.Time{}
	}
	_ = c.db.WithContext(ctx).Model(cred).Updates(updates).Error
}

// VerifyAndChangePassword verifies the current credentials and changes the password
// atomically. Used for the must-change-on-first-login flow.
func (c *Client) VerifyAndChangePassword(ctx context.Context, username, currentPassword, newPassword string) (*types.User, error) {
	user, mustChange, err := c.VerifyLocalCredential(ctx, username, currentPassword)
	if err != nil {
		return nil, err
	}
	if !mustChange {
		return nil, errors.New("password change is not required")
	}
	if err := c.UpdateLocalPassword(ctx, user.ID, newPassword, false); err != nil {
		return nil, err
	}
	return user, nil
}

// UpdateLocalPassword replaces the bcrypt hash for the given user.
// Pass mustChange=false to clear the flag (user self-service), true to keep it set (admin reset).
func (c *Client) UpdateLocalPassword(ctx context.Context, userID uint, newPlainPassword string, mustChange bool) error {
	return c.saveLocalCredential(ctx, userID, newPlainPassword, mustChange)
}

// HasLocalCredential reports whether the given user has a local password set.
func (c *Client) HasLocalCredential(ctx context.Context, userID uint) (bool, error) {
	_, err := c.getLocalCredential(ctx, userID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	return err == nil, err
}

// UnlockLocalUser clears the Disabled flag and any active lockout for the given user.
// Returns an error if the user has no local credential (i.e. is not a local-auth account).
func (c *Client) UnlockLocalUser(ctx context.Context, userID uint) error {
	result := c.db.WithContext(ctx).
		Model(&types.LocalCredential{}).
		Where("user_id = ?", userID).
		Updates(map[string]any{
			"disabled":     false,
			"locked_until": time.Time{},
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("user %d has no local credential", userID)
	}
	return nil
}

// DisableInactiveLocalUsers disables local accounts not logged in within inactiveDuration.
// Accounts that have never logged in are also disabled once the threshold passes,
// with the 30-day clock starting from account creation (not from a NULL origin).
func (c *Client) DisableInactiveLocalUsers(ctx context.Context, inactiveDuration time.Duration) error {
	cutoff := time.Now().Add(-inactiveDuration)
	// Use COALESCE(last_login_at, created_at) so that fresh accounts that have never
	// logged in are measured from their creation time, not from a NULL baseline that
	// would immediately satisfy the condition on day one.
	return c.db.WithContext(ctx).
		Model(&types.LocalCredential{}).
		Where("disabled = false AND COALESCE(last_login_at, created_at) < ?", cutoff).
		Update("disabled", true).Error
}

// DisabledLocalUserIDs returns the set of user IDs whose local credential is currently disabled.
// Used to annotate the users list response with the LocalAuthDisabled flag.
func (c *Client) DisabledLocalUserIDs(ctx context.Context, userIDs []uint) (map[uint]bool, error) {
	if len(userIDs) == 0 {
		return map[uint]bool{}, nil
	}
	var creds []types.LocalCredential
	if err := c.db.WithContext(ctx).
		Select("user_id").
		Where("user_id IN ? AND disabled = true", userIDs).
		Find(&creds).Error; err != nil {
		return nil, err
	}
	disabled := make(map[uint]bool, len(creds))
	for _, cr := range creds {
		disabled[cr.UserID] = true
	}
	return disabled, nil
}

// ---- internal helpers ----

// saveLocalCredential validates the password, hashes it with bcrypt, optionally
// encrypts the hash at rest, and upserts it into the local_credentials table.
func (c *Client) saveLocalCredential(ctx context.Context, userID uint, plainPassword string, mustChange bool) error {
	if err := ValidatePasswordComplexity(plainPassword); err != nil {
		return types2.NewErrHTTP(http.StatusBadRequest, err.Error())
	}
	hashed, err := bcrypt.GenerateFromPassword([]byte(plainPassword), bcryptCost)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}
	cred := &types.LocalCredential{
		UserID:       userID,
		PasswordHash: string(hashed),
		MustChange:   mustChange,
	}
	if err := c.encryptLocalCredential(ctx, cred); err != nil {
		return fmt.Errorf("failed to encrypt local credential: %w", err)
	}
	return c.db.WithContext(ctx).Save(cred).Error
}

func (c *Client) recordFailedAttempt(ctx context.Context, cred *types.LocalCredential) {
	cred.FailedAttempts++
	updates := map[string]any{
		"failed_attempts": cred.FailedAttempts,
	}
	if cred.FailedAttempts >= maxFailedAttempts {
		// Exponential backoff: baseLockout * 2^(excess failures), capped at 2^10.
		multiplier := 1 << min(cred.FailedAttempts-maxFailedAttempts, 10)
		cred.LockedUntil = time.Now().Add(baseLockoutDuration * time.Duration(multiplier))
		updates["locked_until"] = cred.LockedUntil
	}
	// DB error is intentionally ignored: the in-memory cred state is still updated,
	// so the lockout is enforced for the duration of this process even if the write fails.
	_ = c.db.WithContext(ctx).Model(cred).Updates(updates).Error
}

func (c *Client) getLocalCredential(ctx context.Context, userID uint) (*types.LocalCredential, error) {
	cred := new(types.LocalCredential)
	if err := c.db.WithContext(ctx).Where("user_id = ?", userID).First(cred).Error; err != nil {
		return nil, err
	}
	return cred, nil
}

func (c *Client) encryptLocalCredential(ctx context.Context, cred *types.LocalCredential) error {
	if c.encryptionConfig == nil {
		return nil
	}
	transformer := c.encryptionConfig.Transformers[localCredGroupResource]
	if transformer == nil {
		return nil
	}

	b, err := transformer.TransformToStorage(ctx, []byte(cred.PasswordHash), localCredDataCtx(cred.UserID))
	if err != nil {
		return err
	}
	cred.PasswordHash = base64.StdEncoding.EncodeToString(b)
	cred.Encrypted = true
	return nil
}

func (c *Client) decryptLocalCredential(ctx context.Context, userID uint, cred *types.LocalCredential) error {
	if !cred.Encrypted || c.encryptionConfig == nil {
		return nil
	}
	transformer := c.encryptionConfig.Transformers[localCredGroupResource]
	if transformer == nil {
		return nil
	}

	decoded := make([]byte, base64.StdEncoding.DecodedLen(len(cred.PasswordHash)))
	n, err := base64.StdEncoding.Decode(decoded, []byte(cred.PasswordHash))
	if err != nil {
		return err
	}

	out, _, err := transformer.TransformFromStorage(ctx, decoded[:n], localCredDataCtx(userID))
	if err != nil {
		return err
	}
	cred.PasswordHash = string(out)
	return nil
}

func localCredDataCtx(userID uint) value.Context {
	return value.DefaultContext(fmt.Sprintf("%s/%d", localCredGroupResource.String(), userID))
}

// FormatUserID converts a uint user ID to its string representation.
func FormatUserID(id uint) string {
	return strconv.FormatUint(uint64(id), 10)
}
