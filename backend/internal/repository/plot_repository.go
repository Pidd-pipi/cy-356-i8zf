package repository

import (
	"errors"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/communitygarden/server/internal/model"
	"github.com/communitygarden/server/internal/util"
)

// PlotRepository 地块仓储接口。
type PlotRepository interface {
	Create(p *model.Plot) error
	Update(p *model.Plot) error
	FindByID(id uint) (*model.Plot, error)
	FindByIDForUpdate(tx *gorm.DB, id uint) (*model.Plot, error)
	UpdateWithTx(tx *gorm.DB, p *model.Plot) error
	FindByCode(code string) (*model.Plot, error)
	List(pq util.PageQuery, status string) ([]model.Plot, int64, error)
	CountByStatus() (map[string]int64, error)
}

type plotRepository struct {
	db *gorm.DB
}

// NewPlotRepository 构造地块仓储。
func NewPlotRepository(db *gorm.DB) PlotRepository {
	return &plotRepository{db: db}
}

func (r *plotRepository) Create(p *model.Plot) error {
	return r.db.Create(p).Error
}

func (r *plotRepository) Update(p *model.Plot) error {
	return r.db.Save(p).Error
}

func (r *plotRepository) FindByID(id uint) (*model.Plot, error) {
	var p model.Plot
	if err := r.db.Preload("Adopter").First(&p, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	caretaker, err := r.findOpenCaretaker(id)
	if err != nil {
		return nil, err
	}
	p.Caretaker = caretaker
	return &p, nil
}

// plotOpenCaretakerStatuses 与 plotCaretakerRepository 保持一致（invited/active）。
var plotOpenCaretakerStatuses = []string{"invited", "active"}

// findOpenCaretaker 查询地块当前非终态共管记录（按 id 倒序取最新一条）。
func (r *plotRepository) findOpenCaretaker(plotID uint) (*model.PlotCaretaker, error) {
	var c model.PlotCaretaker
	if err := r.db.Preload("Caretaker").Preload("Inviter").
		Where("plot_id = ? AND status IN ?", plotID, plotOpenCaretakerStatuses).
		Order("id DESC").First(&c).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &c, nil
}

// attachCaretakers 批量为地块列表组装当前共管记录。
func (r *plotRepository) attachCaretakers(plots []model.Plot) error {
	if len(plots) == 0 {
		return nil
	}
	ids := make([]uint, 0, len(plots))
	for i := range plots {
		ids = append(ids, plots[i].ID)
	}
	var caretakers []model.PlotCaretaker
	if err := r.db.Preload("Caretaker").Preload("Inviter").
		Where("plot_id IN ? AND status IN ?", ids, plotOpenCaretakerStatuses).
		Order("id DESC").Find(&caretakers).Error; err != nil {
		return err
	}
	latest := make(map[uint]*model.PlotCaretaker, len(caretakers))
	for i := range caretakers {
		if _, exists := latest[caretakers[i].PlotID]; !exists {
			c := caretakers[i]
			latest[caretakers[i].PlotID] = &c
		}
	}
	for i := range plots {
		plots[i].Caretaker = latest[plots[i].ID]
	}
	return nil
}

// FindByIDForUpdate 并发认养使用 SELECT ... FOR UPDATE 行锁（事务内执行）。
func (r *plotRepository) FindByIDForUpdate(tx *gorm.DB, id uint) (*model.Plot, error) {
	var p model.Plot
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&p, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &p, nil
}

// UpdateWithTx 在指定事务内更新地块。
func (r *plotRepository) UpdateWithTx(tx *gorm.DB, p *model.Plot) error {
	return tx.Save(p).Error
}

func (r *plotRepository) FindByCode(code string) (*model.Plot, error) {
	var p model.Plot
	if err := r.db.Where("code = ?", code).First(&p).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &p, nil
}

func (r *plotRepository) List(pq util.PageQuery, status string) ([]model.Plot, int64, error) {
	var plots []model.Plot
	var total int64
	q := r.db.Model(&model.Plot{}).Preload("Adopter")
	if status != "" {
		q = q.Where("status = ?", status)
	}
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := util.Paginate(q.Order("id ASC"), pq).Find(&plots).Error; err != nil {
		return nil, 0, err
	}
	if err := r.attachCaretakers(plots); err != nil {
		return nil, 0, err
	}
	return plots, total, nil
}

func (r *plotRepository) CountByStatus() (map[string]int64, error) {
	type row struct {
		Status string
		Count  int64
	}
	var rows []row
	if err := r.db.Model(&model.Plot{}).Select("status, count(*) as count").Group("status").Scan(&rows).Error; err != nil {
		return nil, err
	}
	out := make(map[string]int64, len(rows))
	for _, v := range rows {
		out[v.Status] = v.Count
	}
	return out, nil
}
