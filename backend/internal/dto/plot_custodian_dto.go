package dto

import (
	"time"

	"github.com/communitygarden/server/internal/constants"
	"github.com/communitygarden/server/internal/model"
)

// CreateCustodianInviteRequest 认养人邀请共管人（按用户名邀请注册用户）。
type CreateCustodianInviteRequest struct {
	Username string `json:"username" binding:"required,max=64"`
}

// PlotCustodianOutDTO 地块共管记录输出（邀请/接受/移除后均返回，供地块列表展示）。
type PlotCustodianOutDTO struct {
	ID          uint        `json:"id"`
	PlotID      uint        `json:"plot_id"`
	CustodianID uint        `json:"custodian_id"`
	Custodian   *UserOutDTO `json:"custodian"`
	InviterID   uint        `json:"inviter_id"`
	Inviter     *UserOutDTO `json:"inviter"`
	Status      string      `json:"status"`
	InvitedAt   string      `json:"invited_at"`
	AcceptedAt  *string     `json:"accepted_at"`
	RemovedAt   *string     `json:"removed_at"`
	CreatedAt   string      `json:"created_at"`
}

// ToPlotCustodianOutDTO 模型转 DTO。
func ToPlotCustodianOutDTO(c *model.PlotCustodian) *PlotCustodianOutDTO {
	out := &PlotCustodianOutDTO{
		ID:          c.ID,
		PlotID:      c.PlotID,
		CustodianID: c.CustodianID,
		InviterID:   c.InviterID,
		Status:      c.Status,
		InvitedAt:   formatCustodianTime(c.InvitedAt),
		CreatedAt:   c.CreatedAt.Format("2006-01-02 15:04:05"),
	}
	if c.AcceptedAt != nil {
		s := formatCustodianTime(*c.AcceptedAt)
		out.AcceptedAt = &s
	}
	if c.RemovedAt != nil {
		s := formatCustodianTime(*c.RemovedAt)
		out.RemovedAt = &s
	}
	if c.Custodian != nil {
		out.Custodian = ToUserOutDTO(c.Custodian)
	}
	if c.Inviter != nil {
		out.Inviter = ToUserOutDTO(c.Inviter)
	}
	return out
}

func formatCustodianTime(t time.Time) string {
	return t.Format("2006-01-02 15:04:05")
}

// AttachCustodians 为地块 DTO 附加当前共管人与最近一次共管记录（地块列表/详情复用）。
// history 按 id DESC 排序，首条即最近一次邀请（含 pending/accepted/removed）。
func (p *PlotOutDTO) AttachCustodians(history []model.PlotCustodian) {
	for i := range history {
		c := &history[i]
		if c.Status == string(constants.CustodianAccepted) {
			p.Custodian = ToPlotCustodianOutDTO(c)
			break
		}
	}
	if len(history) > 0 {
		p.LatestCustodian = ToPlotCustodianOutDTO(&history[0])
	}
}
