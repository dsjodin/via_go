package model

import (
	"time"
)

// UserForm is the part of a user a client may read and write freely.
//
// As with Group, the password is deliberately not here: it lives on User with
// a "-" JSON tag so no response can carry it, and on UserRequest so it can
// still be supplied on writes. What is stored is a bcrypt hash, but a hash is
// still worth cracking offline and there is no reason to hand it out.
type UserForm struct {
	Username string `json:"username" gorm:"type:varchar(255)"`
	Email    string `json:"email" gorm:"type:varchar(255)"`
	Comment  string `json:"comment" gorm:"type:varchar(255)"`
}

// UserRequest is the shape accepted when creating or updating a user.
type UserRequest struct {
	UserForm

	Password string `json:"password"`
}

type User struct {
	ID int `json:"id" gorm:"primary_key"`

	UserForm

	// Password is stored as a bcrypt hash.
	Password string `json:"-" gorm:"type:varchar(255)"`

	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	DeletedAt *time.Time `json:"deleted_at,omitempty"`
}
