package database

import "time"

type (
	Status int

	Report struct {
		ID uint64 `gorm:"primarykey"`

		Config string `gorm:"size:32;not null;default:default"`

		ClientSteamID uint64 `gorm:"index;not null"`
		ClientIP      string `gorm:"size:46;not null"`
		TargetSteamID uint64 `gorm:"index"`
		TargetIP      string `gorm:"size:46"`

		ServerIP string `gorm:"size:46;not null"`
		Reason   string `gorm:"size:256;not null"`

		AdminID   uint64 `gorm:"index"`
		ChannelID uint64 `gorm:"index:idx_reports_message,unique"`
		MessageID uint64 `gorm:"index:idx_reports_message,unique"`
		Status    Status `gorm:"type:TINYINT UNSIGNED NOT NULL;default:0"`

		Comments []Comment `gorm:"foreignKey:ReportID"`

		CreatedAt time.Time `gorm:"not null;default:CURRENT_TIMESTAMP"`
	}

	Comment struct {
		ID uint64 `gorm:"primarykey"`

		ReportID uint64 `gorm:"not null"`
		AdminID  uint64 `gorm:"index;not null"`
		Text     string `gorm:"size:256;not null"`

		CreatedAt time.Time `gorm:"not null;default:CURRENT_TIMESTAMP"`
	}
)

const (
	StatusUnknown Status = iota
	StatusVerified
	StatusFalsified
	StatusAutoClosed
)

func (status Status) String() string {
	switch status {
	case StatusVerified:
		return "Verified"
	case StatusFalsified:
		return "Falsified"
	case StatusAutoClosed:
		return "Auto Closed"
	default:
		return "Unknown"
	}
}
