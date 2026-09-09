package accounts

type Account struct {
	ID       string `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	UserID   string `gorm:"type:uuid;not null;uniqueIndex"`
	Balance  int64  `gorm:"type:bigint;default:0"`
	Currency string `gorm:"size:3;default:'RUB'"`
}
