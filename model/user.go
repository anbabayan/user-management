package model

// todo check data types, indexes on foreign key
type User struct {
	ID       string    `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	Avatar   string    `json:"avatar"`
	Username string    `gorm:"unique;not null" json:"username"`
	Name     string    `json:"name"`
	Password string    `gorm:"not null" json:"password"`
	Status   string    `gorm:"not null;check:status IN ('ACTIVE','BLOCKED')" json:"status"`
	Contacts []Contact `gorm:"foreignKey:UserID" json:"contacts"`
}

type Contact struct {
	ID          string `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	UserID      string `gorm:"type:uuid;not null" json:"user_id"`
	ContactType string `gorm:"not null;check:contact_type IN ('PHONE','WORK','WHATSAPP')" json:"contact_type"`
	Value       string `gorm:"not null" json:"value"`
}
