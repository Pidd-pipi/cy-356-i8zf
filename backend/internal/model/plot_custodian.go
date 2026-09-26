package model

import "time"

// PlotCustodian 地块共管邀请实体（认养人邀请一位注册用户帮忙写种植日记）。
// 状态机：pending -> accepted -> removed（pending 也可直接 removed）。
// 同一次认养周期内同一地块至多存在一条 pending/accepted 记录（service 事务 + 行锁保证）。
type PlotCustodian struct {
	ID          uint       `gorm:"primaryKey" json:"id"`
	PlotID      uint       `gorm:"index;not null" json:"plot_id"`
	Plot        *Plot      `gorm:"foreignKey:PlotID" json:"plot"`
	CustodianID uint       `gorm:"index;not null" json:"custodian_id"`
	Custodian   *User      `gorm:"foreignKey:CustodianID" json:"custodian"`
	InviterID   uint       `gorm:"index;not null" json:"inviter_id"`
	Inviter     *User      `gorm:"foreignKey:InviterID" json:"inviter"`
	Status      string     `gorm:"size:32;not null;default:pending;index" json:"status"`
	InvitedAt   time.Time  `gorm:"not null" json:"invited_at"`
	AcceptedAt  *time.Time `json:"accepted_at"`
	RemovedAt   *time.Time `json:"removed_at"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// TableName 显式指定共管表名（GORM 默认复数 plot_custodians 一致，显式声明便于迁移对齐）。
func (PlotCustodian) TableName() string {
	return "plot_custodians"
}
