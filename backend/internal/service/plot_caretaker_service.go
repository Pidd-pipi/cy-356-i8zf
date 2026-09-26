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

// PlotCaretakerService 地块共管服务（邀请/接受/移除状态机）。
type PlotCaretakerService struct {
	caretakerRepo repository.PlotCaretakerRepository
	userRepo      repository.UserRepository
	plotSvc       *PlotService
	db            *gorm.DB
	logger        *slog.Logger
}

// NewPlotCaretakerService 构造地块共管服务。
func NewPlotCaretakerService(caretakerRepo repository.PlotCaretakerRepository, userRepo repository.UserRepository, plotSvc *PlotService, db *gorm.DB, logger *slog.Logger) *PlotCaretakerService {
	return &PlotCaretakerService{caretakerRepo: caretakerRepo, userRepo: userRepo, plotSvc: plotSvc, db: db, logger: logger}
}

// Invite 认养人邀请一位注册用户成为共管人（每地块至多一条非终态记录）。
func (s *PlotCaretakerService) Invite(plotID, inviterID uint, username string) (*model.PlotCaretaker, error) {
	var invited *model.PlotCaretaker
	err := s.db.Transaction(func(tx *gorm.DB) error {
		target, err := s.userRepo.FindByUsernameWithTx(tx, username)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return util.NewAppError(constants.CodeNotFound, 404, fmt.Sprintf("用户名字段 %s 对应用户不存在，请先注册", username))
			}
			return util.NewAppError(constants.CodeInternalError, 500, constants.ErrorText[constants.CodeInternalError]).Wrap(err)
		}
		if target.Status != string(constants.UserStatusActive) {
			return util.NewAppError(constants.CodeUserDisabled, 409, fmt.Sprintf("用户 %s 账号已被禁用，无法邀请为共管人", username))
		}
		if target.ID == inviterID {
			return util.NewAppError(constants.CodeCaretakerConflict, 409, "认养人不能邀请自己成为共管人")
		}
		plot, err := s.plotSvc.getByIDForUpdate(tx, plotID)
		if err != nil {
			return err
		}
		if plot.AdopterID == nil || *plot.AdopterID != inviterID {
			return util.NewAppError(constants.CodeForbidden, 403, fmt.Sprintf("仅地块 %s 的认养人可邀请共管人", plot.Code))
		}
		if plot.Status != string(constants.PlotStatusAdopted) {
			return util.NewAppError(constants.CodeCaretakerConflict, 409, fmt.Sprintf("地块 %s 当前状态为 %s，不可邀请共管人", plot.Code, util.PlotStatusText(plot.Status)))
		}
		open, err := s.caretakerRepo.FindOpenByPlotForUpdate(tx, plotID)
		if err != nil && !errors.Is(err, repository.ErrNotFound) {
			return util.NewAppError(constants.CodeInternalError, 500, constants.ErrorText[constants.CodeInternalError]).Wrap(err)
		}
		if err == nil {
			if open.Status == string(constants.CaretakerActive) {
				return util.NewAppError(constants.CodeCaretakerConflict, 409, fmt.Sprintf("地块 %s 已有共管人（用户 id=%d），请先移除再邀请", plot.Code, open.CaretakerID))
			}
			return util.NewAppError(constants.CodeCaretakerConflict, 409, fmt.Sprintf("地块 %s 已向用户 id=%d 发出邀请，等待接受中", plot.Code, open.CaretakerID))
		}
		c := &model.PlotCaretaker{
			PlotID:      plotID,
			CaretakerID: target.ID,
			InviterID:   inviterID,
			Status:      string(constants.CaretakerInvited),
		}
		if err := s.caretakerRepo.CreateWithTx(tx, c); err != nil {
			return util.NewAppError(constants.CodeInternalError, 500, constants.ErrorText[constants.CodeInternalError]).Wrap(err)
		}
		invited = c
		return nil
	})
	if err != nil {
		return nil, err
	}
	s.logger.Info(constants.LogCaretakerInvited, "plot_id", plotID, "caretaker_id", invited.CaretakerID, "inviter_id", inviterID)
	return invited, nil
}

// Accept 被邀请人接受共管邀请（invited -> active）。
func (s *PlotCaretakerService) Accept(plotID, caretakerID uint) (*model.PlotCaretaker, error) {
	var accepted *model.PlotCaretaker
	err := s.db.Transaction(func(tx *gorm.DB) error {
		plot, err := s.plotSvc.getByIDForUpdate(tx, plotID)
		if err != nil {
			return err
		}
		open, err := s.caretakerRepo.FindOpenByPlotForUpdate(tx, plotID)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return util.NewAppError(constants.CodeCaretakerNotInvited, 404, fmt.Sprintf("地块 %s 没有待处理的共管邀请", plot.Code))
			}
			return util.NewAppError(constants.CodeInternalError, 500, constants.ErrorText[constants.CodeInternalError]).Wrap(err)
		}
		if open.CaretakerID != caretakerID {
			return util.NewAppError(constants.CodeForbidden, 403, fmt.Sprintf("用户 id=%d 不是地块 %s 的受邀共管人", caretakerID, plot.Code))
		}
		if open.Status != string(constants.CaretakerInvited) {
			return util.NewAppError(constants.CodeCaretakerConflict, 409, fmt.Sprintf("地块 %s 共管状态为 %s，无需重复接受", plot.Code, util.CaretakerStatusText(open.Status)))
		}
		now := time.Now()
		open.Status = string(constants.CaretakerActive)
		open.AcceptedAt = &now
		if err := s.caretakerRepo.UpdateWithTx(tx, open); err != nil {
			return util.NewAppError(constants.CodeInternalError, 500, constants.ErrorText[constants.CodeInternalError]).Wrap(err)
		}
		accepted = open
		return nil
	})
	if err != nil {
		return nil, err
	}
	s.logger.Info(constants.LogCaretakerAccepted, "plot_id", plotID, "caretaker_id", caretakerID, "inviter_id", accepted.InviterID)
	return accepted, nil
}

// Remove 认养人移除共管人（同时用于撤销待接受邀请；invited/active -> ended）。
func (s *PlotCaretakerService) Remove(plotID, operatorID uint) (*model.PlotCaretaker, error) {
	var removed *model.PlotCaretaker
	err := s.db.Transaction(func(tx *gorm.DB) error {
		plot, err := s.plotSvc.getByIDForUpdate(tx, plotID)
		if err != nil {
			return err
		}
		if plot.AdopterID == nil || *plot.AdopterID != operatorID {
			return util.NewAppError(constants.CodeForbidden, 403, fmt.Sprintf("仅地块 %s 的认养人可移除共管人", plot.Code))
		}
		open, err := s.caretakerRepo.FindOpenByPlotForUpdate(tx, plotID)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return util.NewAppError(constants.CodeCaretakerNotInvited, 404, fmt.Sprintf("地块 %s 当前没有共管人或待接受邀请", plot.Code))
			}
			return util.NewAppError(constants.CodeInternalError, 500, constants.ErrorText[constants.CodeInternalError]).Wrap(err)
		}
		now := time.Now()
		open.Status = string(constants.CaretakerEnded)
		open.RemovedAt = &now
		if err := s.caretakerRepo.UpdateWithTx(tx, open); err != nil {
			return util.NewAppError(constants.CodeInternalError, 500, constants.ErrorText[constants.CodeInternalError]).Wrap(err)
		}
		removed = open
		return nil
	})
	if err != nil {
		return nil, err
	}
	s.logger.Info(constants.LogCaretakerRemoved, "plot_id", plotID, "caretaker_id", removed.CaretakerID, "operator", operatorID)
	return removed, nil
}

// ListMyInvitations 查询当前用户收到的待接受邀请。
func (s *PlotCaretakerService) ListMyInvitations(caretakerID uint) ([]model.PlotCaretaker, error) {
	list, err := s.caretakerRepo.ListPendingByCaretaker(caretakerID)
	if err != nil {
		return nil, util.NewAppError(constants.CodeInternalError, 500, constants.ErrorText[constants.CodeInternalError]).Wrap(err)
	}
	s.logger.Info(constants.LogCaretakerListed, "user_id", caretakerID, "total", len(list))
	return list, nil
}

// EndOpenByPlot 地块释放事务内终止全部共管关系（由 PlotService.Release 通过回调复用）。
func (s *PlotCaretakerService) EndOpenByPlot(tx *gorm.DB, plotID uint) error {
	return s.caretakerRepo.EndAllOpenByPlotWithTx(tx, plotID)
}
