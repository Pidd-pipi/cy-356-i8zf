package repository

import (
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/communitygarden/server/internal/model"
)

// PlotCaretakerRepository 地块共管人仓储接口。
type PlotCaretakerRepository interface {
	Create(c *model.PlotCaretaker) error
	CreateWithTx(tx *gorm.DB, c *model.PlotCaretaker) error
	Update(c *model.PlotCaretaker) error
	UpdateWithTx(tx *gorm.DB, c *model.PlotCaretaker) error
	FindByID(id uint) (*model.PlotCaretaker, error)
	FindByIDForUpdate(tx *gorm.DB, id uint) (*model.PlotCaretaker, error)
	FindOpenByPlot(plotID uint) (*model.PlotCaretaker, error)
	FindOpenByPlotForUpdate(tx *gorm.DB, plotID uint) (*model.PlotCaretaker, error)
	FindActiveByPlot(plotID uint) (*model.PlotCaretaker, error)
	FindActiveByPlots(plotIDs []uint) ([]model.PlotCaretaker, error)
	ListPendingByCaretaker(caretakerID uint) ([]model.PlotCaretaker, error)
	EndAllOpenByPlotWithTx(tx *gorm.DB, plotID uint) error
}

type plotCaretakerRepository struct {
	db *gorm.DB
}

// NewPlotCaretakerRepository 构造地块共管人仓储。
func NewPlotCaretakerRepository(db *gorm.DB) PlotCaretakerRepository {
	return &plotCaretakerRepository{db: db}
}

// openStatuses 非终态状态：已邀请、共管中。
var openCaretakerStatuses = []string{"invited", "active"}

func (r *plotCaretakerRepository) Create(c *model.PlotCaretaker) error {
	return r.db.Create(c).Error
}

func (r *plotCaretakerRepository) CreateWithTx(tx *gorm.DB, c *model.PlotCaretaker) error {
	return tx.Create(c).Error
}

func (r *plotCaretakerRepository) Update(c *model.PlotCaretaker) error {
	return r.db.Save(c).Error
}

func (r *plotCaretakerRepository) UpdateWithTx(tx *gorm.DB, c *model.PlotCaretaker) error {
	return tx.Save(c).Error
}

func (r *plotCaretakerRepository) FindByID(id uint) (*model.PlotCaretaker, error) {
	var c model.PlotCaretaker
	if err := r.db.Preload("Plot").Preload("Caretaker").Preload("Inviter").First(&c, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &c, nil
}

func (r *plotCaretakerRepository) FindByIDForUpdate(tx *gorm.DB, id uint) (*model.PlotCaretaker, error) {
	var c model.PlotCaretaker
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&c, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &c, nil
}

func (r *plotCaretakerRepository) FindOpenByPlot(plotID uint) (*model.PlotCaretaker, error) {
	return r.findOpenByPlot(r.db, plotID, true)
}

// FindOpenByPlotForUpdate 事务内行锁查询；不做 Preload，避免事务内嵌套查询
// （服务层事务内只需要状态与用户 ID 标量字段，Preload 在事务外补载）。
func (r *plotCaretakerRepository) FindOpenByPlotForUpdate(tx *gorm.DB, plotID uint) (*model.PlotCaretaker, error) {
	return r.findOpenByPlot(tx.Clauses(clause.Locking{Strength: "UPDATE"}), plotID, false)
}

// findOpenByPlot 查询地块当前非终态共管记录（每地块至多一条，按 id 倒序兜底取最新）。
func (r *plotCaretakerRepository) findOpenByPlot(q *gorm.DB, plotID uint, preload bool) (*model.PlotCaretaker, error) {
	var c model.PlotCaretaker
	query := q.Where("plot_id = ? AND status IN ?", plotID, openCaretakerStatuses).Order("id DESC")
	if preload {
		query = query.Preload("Caretaker").Preload("Inviter")
	}
	if err := query.First(&c).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &c, nil
}

func (r *plotCaretakerRepository) FindActiveByPlot(plotID uint) (*model.PlotCaretaker, error) {
	var c model.PlotCaretaker
	if err := r.db.Where("plot_id = ? AND status = ?", plotID, "active").First(&c).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &c, nil
}

// FindActiveByPlots 批量查询多地片的共管中记录（地块列表组装复用）。
func (r *plotCaretakerRepository) FindActiveByPlots(plotIDs []uint) ([]model.PlotCaretaker, error) {
	var list []model.PlotCaretaker
	if len(plotIDs) == 0 {
		return list, nil
	}
	if err := r.db.Preload("Caretaker").Preload("Inviter").
		Where("plot_id IN ? AND status IN ?", plotIDs, openCaretakerStatuses).
		Order("id DESC").Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

// ListPendingByCaretaker 查询某用户收到的待接受邀请（按 id 倒序）。
func (r *plotCaretakerRepository) ListPendingByCaretaker(caretakerID uint) ([]model.PlotCaretaker, error) {
	var list []model.PlotCaretaker
	if err := r.db.Preload("Plot").Preload("Inviter").
		Where("caretaker_id = ? AND status = ?", caretakerID, "invited").
		Order("id DESC").Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

// EndAllOpenByPlotWithTx 地块释放时，在同一事务内终止全部非终态共管关系。
func (r *plotCaretakerRepository) EndAllOpenByPlotWithTx(tx *gorm.DB, plotID uint) error {
	var rows []model.PlotCaretaker
	if err := tx.Where("plot_id = ? AND status IN ?", plotID, openCaretakerStatuses).Find(&rows).Error; err != nil {
		return err
	}
	now := time.Now()
	for i := range rows {
		rows[i].Status = "ended"
		rows[i].RemovedAt = &now
		if err := tx.Save(&rows[i]).Error; err != nil {
			return err
		}
	}
	return nil
}
