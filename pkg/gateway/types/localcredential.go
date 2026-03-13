//nolint:revive
package types

import "time"

// LocalCredential stores a bcrypt password hash for a local (non-OAuth) user account.
// It is stored in a separate table so that the User struct and its encryption pipeline
// remain unchanged, minimizing upstream merge conflicts.
//
// The password hash itself is also encrypted at rest when an encryption config is present,
// using the same AES-256-GCM envelope that protects User and Identity fields.
type LocalCredential struct {
	// UserID is the primary key, referencing users.id.
	UserID uint `gorm:"primaryKey"`

	// CreatedAt is set by GORM at row creation time.
	// Used as the inactivity baseline for accounts that have never logged in:
	// the 30-day inactivity clock starts from creation, not from a NULL origin.
	CreatedAt time.Time

	// PasswordHash is the bcrypt hash of the user's password.
	// When Encrypted is true, this field is additionally AES-encrypted and base64-encoded.
	PasswordHash string

	// Encrypted indicates whether PasswordHash has been encrypted at rest.
	Encrypted bool

	// MustChange forces the user to change their password on the next login.
	// Set by admins when creating a user or resetting a password.
	MustChange bool

	// FailedAttempts counts consecutive failed login attempts for this account.
	// Reset to zero on a successful authentication.
	FailedAttempts int

	// LockedUntil, when non-zero, prevents authentication until the specified time.
	// Set automatically after repeated failed attempts.
	LockedUntil time.Time

	// Disabled, when true, prevents the account from logging in.
	// Set automatically when the account has been inactive for too long (Dev-7).
	// Cleared by an admin via the unlock endpoint.
	Disabled bool

	// LastLoginAt records the time of the most recent successful login.
	// Used to determine inactivity and trigger automatic account disabling (Dev-7).
	LastLoginAt *time.Time
}
