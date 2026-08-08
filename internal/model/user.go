package model

import "time"

type User struct {
	ID              string     `db:"id"`
	Email           string     `db:"email"`
	PasswordHash    *string    `db:"password_hash"`
	EmailVerifiedAt *time.Time `db:"email_verified_at"`
	FirstName       string     `db:"first_name"`
	LastName        string     `db:"last_name"`
	CreatedAt       time.Time  `db:"created_at"`
}

func (u *User) HasPassword() bool {
	return u != nil && u.PasswordHash != nil && *u.PasswordHash != ""
}
