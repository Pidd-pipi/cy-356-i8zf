package service

import (
	"errors"
	"fmt"
	"log/slog"
	"time"

	"gorm.io/gorm"

	"github.com/communitygarden/server/internal/constants"
	"github.com/communitygarden/server/internal/model"
	"github.com/communitygarden/server/internal/repository"
	"github.com/communitygarden/server/internal/util"
)

// PlotCustodianService 地块共管服务（邀请/接受/移除均在事务 + 行锁中完成）。
type PlotCustodianService struct {
	custodianRepo repository.PlotCustodianRepository
	plotRepo      repository.PlotRepository
	userRepo      repository.UserRepository
	db            *gorm.DB
	logger        *slog.Logger
}

// NewPlotCustodianService 构造地块共管服务。
func NewPlotCustodianService(custodianRepo repository.PlotCustodianRepository, plotRepo repository.PlotRepository, userRepo repository.UserRepository, db *gorm.DB, logger *slog.Logger) *PlotCustodianService {
	return &PlotCustodianService{custodianRepo: custodianRepo, plotRepo: plotRepo, userRepo: userRepo, db: db, logger: logger}
}

// Invite 认养人邀请一位注册用户共管地块。
// 规则：仅当前认养人可邀请；地块须处于认养中；不能邀请自己；被邀请人须为启用的注册用户；
// 同一地块同时只能存在一条有效（pending/accepted）共管记录。
func (s *PlotCustodianService) Invite(plotID, inviterID uint, inviteeUsername string) (*model.PlotCustodian, error) {
	invitee, err := s.userRepo.FindByUsername(inviteeUsername)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, util.NewAppError(constants.CodeNotFound, 404, fmt.Sprintf("用户实体 username=%s 不存在，请先注册", inviteeUsername))
		}
		return nil, util.NewAppError(constants.CodeInternalError, 500, constants.ErrorText[constants.CodeInternalError]).Wrap(err)
	}
	if invitee.Status != string(constants.UserStatusActive) {
		return nil, util.NewAppError(constants.CodeUserDisabled, 409, fmt.Sprintf("用户 %s 账号已被禁用，无法成为共管人", inviteeUsername))
	}
	if invitee.ID == inviterID {
		return nil, util.NewAppError(constants.CodeCustodianNotAllowed, 403, "认养人不能邀请自己成为共管人")
	}

	var created *model.PlotCustodian
	err = s.db.Transaction(func(tx *gorm.DB) error {
		plot, err := s.plotRepo.FindByIDForUpdate(tx, plotID)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return util.NewAppError(constants.CodeNotFound, 404, fmt.Sprintf("地块实体 id=%d 不存在", plotID))
			}
			return util.NewAppError(constants.CodeInternalError, 500, constants.ErrorText[constants.CodeInternalError]).Wrap(err)
		}
		if plot.AdopterID == nil || *plot.AdopterID != inviterID {
			return util.NewAppError(constants.CodeCustodianNotAllowed, 403,
				fmt.Sprintf("用户 id=%d 不是地块 %s 的认养人，无权邀请共管人", inviterID, plot.Code))
		}
		if plot.Status != string(constants.PlotStatusAdopted) {
			return util.NewAppError(constants.CodeCustodianConflict, 409,
				fmt.Sprintf("地块 %s 当前状态为 %s，认养中才可邀请共管人", plot.Code, util.PlotStatusText(plot.Status)))
		}
		active, err := s.custodianRepo.FindActiveByPlotForUpdate(tx, plotID)
		if err != nil && !errors.Is(err, repository.ErrNotFound) {
			return util.NewAppError(constants.CodeInternalError, 500, constants.ErrorText[constants.CodeInternalError]).Wrap(err)
		}
		if active != nil {
			if active.CustodianID == invitee.ID {
				return util.NewAppError(constants.CodeCustodianConflict, 409,
					fmt.Sprintf("已邀请用户 %s 共管地块 %s，邀请状态为 %s", inviteeUsername, plot.Code, util.CustodianStatusText(active.Status)))
			}
			return util.NewAppError(constants.CodeCustodianConflict, 409,
				fmt.Sprintf("地块 %s 已有一位共管人（状态 %s），请先移除后再邀请", plot.Code, util.CustodianStatusText(active.Status)))
		}
		now := time.Now()
		c := &model.PlotCustodian{
			PlotID:      plotID,
			CustodianID: invitee.ID,
			InviterID:   inviterID,
			Status:      string(constants.CustodianPending),
			InvitedAt:   now,
		}
		if err := tx.Create(c).Error; err != nil {
			return util.NewAppError(constants.CodeInternalError, 500, constants.ErrorText[constants.CodeInternalError]).Wrap(err)
		}
		created = c
		return nil
	})
	if err != nil {
		return nil, err
	}
	s.logger.Info(constants.LogCustodianInvited, "custodian_id", created.ID, "plot_id", plotID, "adopter_id", inviterID, "invitee_id", invitee.ID)
	return s.reload(created.ID)
}

// Accept 被邀请人接受共管邀请（仅本人；pending -> accepted）。
func (s *PlotCustodianService) Accept(custodianID, userID uint) (*model.PlotCustodian, error) {
	var accepted *model.PlotCustodian
	err := s.db.Transaction(func(tx *gorm.DB) error {
		c, err := s.custodianRepo.FindByIDForUpdate(tx, custodianID)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return util.NewAppError(constants.CodeCustodianNotFound, 404, fmt.Sprintf("地块共管邀请实体 id=%d 不存在", custodianID))
			}
			return util.NewAppError(constants.CodeInternalError, 500, constants.ErrorText[constants.CodeInternalError]).Wrap(err)
		}
		if c.CustodianID != userID {
			return util.NewAppError(constants.CodeCustodianNotAllowed, 403,
				fmt.Sprintf("用户 id=%d 不是共管邀请 id=%d 的被邀请人，无权接受", userID, custodianID))
		}
		if c.Status != string(constants.CustodianPending) {
			return util.NewAppError(constants.CodeCustodianConflict, 409,
				fmt.Sprintf("共管邀请 id=%d 当前状态为 %s，仅待接受状态可接受", custodianID, util.CustodianStatusText(c.Status)))
		}
		now := time.Now()
		c.Status = string(constants.CustodianAccepted)
		c.AcceptedAt = &now
		if err := tx.Save(c).Error; err != nil {
			return util.NewAppError(constants.CodeInternalError, 500, constants.ErrorText[constants.CodeInternalError]).Wrap(err)
		}
		accepted = c
		return nil
	})
	if err != nil {
		return nil, err
	}
	s.logger.Info(constants.LogCustodianAccepted, "custodian_id", custodianID, "plot_id", accepted.PlotID, "user_id", userID)
	return s.reload(custodianID)
}

// Remove 移除共管人/撤回或拒绝邀请（pending|accepted -> removed）。
// 认养人可移除自己发出的邀请；共管人可拒绝(pending)/退出(accepted)自己收到的邀请。
func (s *PlotCustodianService) Remove(custodianID, operatorID uint) (*model.PlotCustodian, error) {
	var removed *model.PlotCustodian
	err := s.db.Transaction(func(tx *gorm.DB) error {
		c, err := s.custodianRepo.FindByIDForUpdate(tx, custodianID)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return util.NewAppError(constants.CodeCustodianNotFound, 404, fmt.Sprintf("地块共管邀请实体 id=%d 不存在", custodianID))
			}
			return util.NewAppError(constants.CodeInternalError, 500, constants.ErrorText[constants.CodeInternalError]).Wrap(err)
		}
		if c.InviterID != operatorID && c.CustodianID != operatorID {
			return util.NewAppError(constants.CodeCustodianNotAllowed, 403,
				fmt.Sprintf("用户 id=%d 既不是地块 id=%d 的认养人也不是共管人，无权移除共管关系", operatorID, c.PlotID))
		}
		if c.Status == string(constants.CustodianRemoved) {
			return util.NewAppError(constants.CodeCustodianConflict, 409,
				fmt.Sprintf("共管邀请 id=%d 已是 %s 状态", custodianID, util.CustodianStatusText(c.Status)))
		}
		now := time.Now()
		c.Status = string(constants.CustodianRemoved)
		c.RemovedAt = &now
		if err := tx.Save(c).Error; err != nil {
			return util.NewAppError(constants.CodeInternalError, 500, constants.ErrorText[constants.CodeInternalError]).Wrap(err)
		}
		removed = c
		return nil
	})
	if err != nil {
		return nil, err
	}
	s.logger.Info(constants.LogCustodianRemoved, "custodian_id", custodianID, "plot_id", removed.PlotID, "operator", operatorID, "status", removed.Status)
	return s.reload(custodianID)
}

// ListByPlot 查询地块共管历史（地块列表“共管”列 + 历史弹窗复用）。
func (s *PlotCustodianService) ListByPlot(plotID uint) ([]model.PlotCustodian, error) {
	list, err := s.custodianRepo.ListByPlot(plotID)
	if err != nil {
		return nil, util.NewAppError(constants.CodeInternalError, 500, constants.ErrorText[constants.CodeInternalError]).Wrap(err)
	}
	return list, nil
}

// reload 事务提交后重新加载关联用户，输出完整 DTO。
func (s *PlotCustodianService) reload(id uint) (*model.PlotCustodian, error) {
	c, err := s.custodianRepo.FindByID(id)
	if err != nil {
		return nil, util.NewAppError(constants.CodeInternalError, 500, constants.ErrorText[constants.CodeInternalError]).Wrap(err)
	}
	return c, nil
}
