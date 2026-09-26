package model

import "time"

// PlotCaretaker 地块共管人实体（邀请 -> 接受 -> 移除状态机）。
// 一个地块至多保留一条非终态（invited/active）记录；终态记录留存用于地块列表展示历史。
type PlotCaretaker struct {
	ID          uint       `gorm:"primaryKey" json:"id"`
	PlotID      uint       `gorm:"index;not null" json:"plot_id"`
	Plot        *Plot      `gorm:"foreignKey:PlotID" json:"plot"`
	CaretakerID uint       `gorm:"index;not null" json:"caretaker_id"`
	Caretaker   *User      `gorm:"foreignKey:CaretakerID" json:"caretaker"`
	InviterID   uint       `gorm:"index;not null" json:"inviter_id"`
	Inviter     *User      `gorm:"foreignKey:InviterID" json:"inviter"`
	Status      string     `gorm:"size:32;not null;default:invited;index" json:"status"`
	InvitedAt   time.Time  `json:"invited_at"`
	AcceptedAt  *time.Time `json:"accepted_at"`
	RemovedAt   *time.Time `json:"removed_at"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}
