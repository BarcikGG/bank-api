package transfers

import "time"

type OperationType string

const (
	OperationDeposit    OperationType = "deposit"
	OperationWithdrawal OperationType = "withdrawal"
	OperationTransfer   OperationType = "transfer"
)

type Transfer struct {
	ID        string        `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	FromAccID *string       `gorm:"type:uuid;default:null"`
	ToAccID   *string       `gorm:"type:uuid;default:null"`
	Type      OperationType `gorm:"size:20;not null"`
	Currency  string        `gorm:"size:3;not null"`
	Amount    int64         `gorm:"type:bigint;not null"`
	CreatedAt time.Time     `gorm:"index"`
}
