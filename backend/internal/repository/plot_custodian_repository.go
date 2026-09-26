package repository

import (
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/communitygarden/server/internal/constants"
	"github.com/communitygarden/server/internal/model"
)

// PlotCustodianRepository 地块共管仓储接口。
type PlotCustodianRepository interface {
	Create(c *model.PlotCustodian) error
	Update(c *model.PlotCustodian) error
	FindByID(id uint) (*model.PlotCustodian, error)
	FindByIDForUpdate(tx *gorm.DB, id uint) (*model.PlotCustodian, error)
	// FindActiveByPlotForUpdate 事务行锁查询地块当前有效（pending/accepted）的共管记录（至多一条）。
	FindActiveByPlotForUpdate(tx *gorm.DB, plotID uint) (*model.PlotCustodian, error)
	// FindAcceptedByPlot 查询地块当前 accepted 共管记录（非事务，日记写权限校验复用）。
	FindAcceptedByPlot(plotID uint) (*model.PlotCustodian, error)
	// ListByPlot 查询地块全部共管记录（最新在前），用于地块列表的共管历史。
	ListByPlot(plotID uint) ([]model.PlotCustodian, error)
	// OverviewByPlotIDs 批量查询多个地块的全部共管记录（最新在前），地块列表 enrichment 复用。
	OverviewByPlotIDs(plotIDs []uint) (map[uint][]model.PlotCustodian, error)
	// RevokeAllByPlotWithTx 释放地块时在同一事务内撤销该地块全部有效（pending/accepted）共管记录。
	RevokeAllByPlotWithTx(tx *gorm.DB, plotID uint) error
	// ListCollaborativePlotIDs 查询用户可协作的地块 ID 集合：本人认养的地块 + 本人 accepted 共管的地块。
	ListCollaborativePlotIDs(userID uint) ([]uint, error)
}

type plotCustodianRepository struct {
	db *gorm.DB
}

// NewPlotCustodianRepository 构造地块共管仓储。
func NewPlotCustodianRepository(db *gorm.DB) PlotCustodianRepository {
	return &plotCustodianRepository{db: db}
}

func (r *plotCustodianRepository) Create(c *model.PlotCustodian) error {
	return r.db.Create(c).Error
}

func (r *plotCustodianRepository) Update(c *model.PlotCustodian) error {
	return r.db.Save(c).Error
}

func (r *plotCustodianRepository) FindByID(id uint) (*model.PlotCustodian, error) {
	var c model.PlotCustodian
	if err := r.db.Preload("Plot").Preload("Custodian").Preload("Inviter").First(&c, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &c, nil
}

// FindByIDForUpdate 共管接受/移除并发使用 SELECT ... FOR UPDATE 行锁（事务内执行）。
func (r *plotCustodianRepository) FindByIDForUpdate(tx *gorm.DB, id uint) (*model.PlotCustodian, error) {
	var c model.PlotCustodian
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&c, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &c, nil
}

func (r *plotCustodianRepository) FindActiveByPlotForUpdate(tx *gorm.DB, plotID uint) (*model.PlotCustodian, error) {
	var c model.PlotCustodian
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("plot_id = ? AND status IN ?", plotID, []string{string(constants.CustodianPending), string(constants.CustodianAccepted)}).
		First(&c).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &c, nil
}

func (r *plotCustodianRepository) FindAcceptedByPlot(plotID uint) (*model.PlotCustodian, error) {
	var c model.PlotCustodian
	err := r.db.Preload("Custodian").
		Where("plot_id = ? AND status = ?", plotID, string(constants.CustodianAccepted)).
		First(&c).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &c, nil
}

func (r *plotCustodianRepository) ListByPlot(plotID uint) ([]model.PlotCustodian, error) {
	var list []model.PlotCustodian
	if err := r.db.Preload("Custodian").Preload("Inviter").
		Where("plot_id = ?", plotID).Order("id DESC").Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

func (r *plotCustodianRepository) OverviewByPlotIDs(plotIDs []uint) (map[uint][]model.PlotCustodian, error) {
	out := make(map[uint][]model.PlotCustodian)
	if len(plotIDs) == 0 {
		return out, nil
	}
	var list []model.PlotCustodian
	if err := r.db.Preload("Custodian").Preload("Inviter").
		Where("plot_id IN ?", plotIDs).Order("id DESC").Find(&list).Error; err != nil {
		return nil, err
	}
	for i := range list {
		out[list[i].PlotID] = append(out[list[i].PlotID], list[i])
	}
	return out, nil
}

func (r *plotCustodianRepository) RevokeAllByPlotWithTx(tx *gorm.DB, plotID uint) error {
	return tx.Model(&model.PlotCustodian{}).
		Where("plot_id = ? AND status IN ?", plotID, []string{string(constants.CustodianPending), string(constants.CustodianAccepted)}).
		Updates(map[string]interface{}{"status": string(constants.CustodianRemoved), "removed_at": time.Now()}).Error
}

func (r *plotCustodianRepository) ListCollaborativePlotIDs(userID uint) ([]uint, error) {
	adoptedSub := r.db.Model(&model.Plot{}).Select("id").Where("adopter_id = ?", userID)
	custodianSub := r.db.Model(&model.PlotCustodian{}).Select("plot_id").
		Where("custodian_id = ? AND status = ?", userID, string(constants.CustodianAccepted))

	var ids []uint
	if err := r.db.Model(&model.Plot{}).
		Distinct("id").
		Where("id IN (?) OR id IN (?)", adoptedSub, custodianSub).
		Pluck("id", &ids).Error; err != nil {
		return nil, err
	}
	return ids, nil
}
