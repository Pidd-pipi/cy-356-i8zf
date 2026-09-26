package dto

import (
	"github.com/communitygarden/server/internal/model"
	"github.com/communitygarden/server/internal/util"
)

// InviteCaretakerRequest 认养人邀请共管人（按用户名邀请注册用户）。
type InviteCaretakerRequest struct {
	Username string `json:"username" binding:"required,max=64"`
}

// PlotCaretakerOutDTO 地块共管记录输出（地块列表内嵌 + 邀请列表复用）。
type PlotCaretakerOutDTO struct {
	ID          uint        `json:"id"`
	PlotID      uint        `json:"plot_id"`
	CaretakerID uint        `json:"caretaker_id"`
	Caretaker   *UserOutDTO `json:"caretaker"`
	InviterID   uint        `json:"inviter_id"`
	Inviter     *UserOutDTO `json:"inviter"`
	Status      string      `json:"status"`
	StatusText  string      `json:"status_text"`
	InvitedAt   string      `json:"invited_at"`
	AcceptedAt  string      `json:"accepted_at"`
	RemovedAt   string      `json:"removed_at"`
}

// CaretakerInvitationDTO 我的待接受邀请列表项（携带地块摘要）。
type CaretakerInvitationDTO struct {
	ID         uint        `json:"id"`
	PlotID     uint        `json:"plot_id"`
	PlotCode   string      `json:"plot_code"`
	PlotName   string      `json:"plot_name"`
	InviterID  uint        `json:"inviter_id"`
	Inviter    *UserOutDTO `json:"inviter"`
	Status     string      `json:"status"`
	StatusText string      `json:"status_text"`
	InvitedAt  string      `json:"invited_at"`
}

// ToCaretakerInvitationDTO 模型转邀请列表 DTO。
func ToCaretakerInvitationDTO(c *model.PlotCaretaker) *CaretakerInvitationDTO {
	out := &CaretakerInvitationDTO{
		ID:         c.ID,
		PlotID:     c.PlotID,
		InviterID:  c.InviterID,
		Status:     c.Status,
		StatusText: util.CaretakerStatusText(c.Status),
		InvitedAt:  c.InvitedAt.Format("2006-01-02 15:04:05"),
	}
	if c.Plot != nil {
		out.PlotCode = c.Plot.Code
		out.PlotName = c.Plot.Name
	}
	if c.Inviter != nil {
		out.Inviter = ToUserOutDTO(c.Inviter)
	}
	return out
}

// ToPlotCaretakerOutDTO 模型转 DTO。
func ToPlotCaretakerOutDTO(c *model.PlotCaretaker) *PlotCaretakerOutDTO {
	out := &PlotCaretakerOutDTO{
		ID:          c.ID,
		PlotID:      c.PlotID,
		CaretakerID: c.CaretakerID,
		InviterID:   c.InviterID,
		Status:      c.Status,
		StatusText:  util.CaretakerStatusText(c.Status),
		InvitedAt:   c.InvitedAt.Format("2006-01-02 15:04:05"),
	}
	if c.Caretaker != nil {
		out.Caretaker = ToUserOutDTO(c.Caretaker)
	}
	if c.Inviter != nil {
		out.Inviter = ToUserOutDTO(c.Inviter)
	}
	if c.AcceptedAt != nil {
		out.AcceptedAt = c.AcceptedAt.Format("2006-01-02 15:04:05")
	}
	if c.RemovedAt != nil {
		out.RemovedAt = c.RemovedAt.Format("2006-01-02 15:04:05")
	}
	return out
}
