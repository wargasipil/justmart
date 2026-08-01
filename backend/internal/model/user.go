package model

import "time"

type User struct {
	ID           string `gorm:"primaryKey;type:uuid;default:gen_random_uuid()"`
	Email        string `gorm:"uniqueIndex;not null"`
	Name         string `gorm:"not null;default:''"`
	PasswordHash string `gorm:"not null;column:password_hash"`
	Role         string `gorm:"not null;index"`
	Active       bool   `gorm:"not null;default:true"`
	// Nil = no profile picture. Denormalized from user_avatars so the common
	// user reads can tell the UI whether to fetch bytes without joining a BLOB.
	AvatarUpdatedAt *time.Time `gorm:"column:avatar_updated_at"`
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func (User) TableName() string { return "users" }

// UserAvatar holds the two renditions of a user's profile picture. Separate
// from `users` on purpose — see migration 00050. ImageData is the bounded
// original; ThumbData is the small square every avatar/list surface reads.
type UserAvatar struct {
	UserID      string `gorm:"primaryKey;column:user_id"`
	ContentType string `gorm:"not null"`
	ImageData   []byte `gorm:"not null"`
	ThumbData   []byte `gorm:"not null"`
	UpdatedAt   time.Time
}

func (UserAvatar) TableName() string { return "user_avatars" }
