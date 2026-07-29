package model

import (
	"time"

	"gorm.io/datatypes"
)

// GroupForm is the part of a group a client may read and write freely.
//
// The ESXi root password is deliberately not here. It lives on Group with a
// "-" JSON tag so it can never be serialised into a response, and on
// GroupRequest so it can still be supplied on writes.
type GroupForm struct {
	Name        string         `json:"name" gorm:"type:varchar(255)"`
	DNS         string         `json:"dns" gorm:"type:varchar(255)"`
	NTP         string         `json:"ntp" gorm:"type:varchar(255)"`
	Netmask     string         `json:"netmask" gorm:"type:varchar(255)"`
	Gateway     string         `json:"gateway" gorm:"type:varchar(255)"`
	Device      string         `json:"device" gorm:"type:varchar(255)"`
	ImageID     int            `json:"image_id" gorm:"type:INT"`
	Ks          string         `json:"ks" gorm:"type:text"`
	Syslog      string         `json:"syslog" gorm:"type:varchar(255)"`
	Vlan        string         `json:"vlan" gorm:"type:INT"`
	CallbackURL string         `json:"callbackurl"`
	BootDisk    string         `json:"bootdisk" gorm:"type:varchar(255)"`
	Options     datatypes.JSON `json:"options" sql:"type:JSONB" swaggertype:"object,string"`
	BootMethod  string         `json:"bootmethod" gorm:"type:varchar(255)"`
}

// GroupRequest is the shape accepted when creating or updating a group.
type GroupRequest struct {
	GroupForm

	// Password is the ESXi root password, in the clear on the way in. It is
	// encrypted before it reaches the database and never sent back out.
	Password string `json:"password"`
}

type Group struct {
	ID int `json:"id" gorm:"primary_key"`

	GroupForm

	// Password is stored AES-256-GCM encrypted. The "-" tag is what keeps it
	// out of every response body; there is no second struct guarding it.
	Password string `json:"-" gorm:"type:varchar(255)"`

	Host []Host `json:"host,omitempty" gorm:"foreignkey:GroupID"`

	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	DeletedAt *time.Time `json:"deleted_at,omitempty"`
}

type GroupOptions struct {
	SSH                  bool `json:"ssh"`
	SuppressShellWarning bool `json:"suppressshellwarning"`
	EraseDisks           bool `json:"erasedisks"`
	Certificate          bool `json:"certificate"`
	CreateVMFS           bool `json:"createvmfs"`
}
