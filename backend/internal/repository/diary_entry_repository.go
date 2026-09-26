package repository

import (
	"errors"

	"gorm.io/gorm"

	"github.com/communitygarden/server/internal/model"
	"github.com/communitygarden/server/internal/util"
)

// DiaryRepository 种植日记仓储接口。
type DiaryRepository interface {
	Create(d *model.DiaryEntry) error
	Update(d *model.DiaryEntry) error
	Delete(id uint) error
	FindByID(id uint) (*model.DiaryEntry, error)
	List(pq util.PageQuery, userID, planID uint) ([]model.DiaryEntry, int64, error)
	ListVisible(pq util.PageQuery, viewerID, planID uint) ([]model.DiaryEntry, int64, error)
	IncrementLike(id uint) error
	CreateComment(c *model.DiaryComment) error
	ListComments(diaryID uint) ([]model.DiaryComment, error)
	CountByUser(userID uint) (int64, error)
}

type diaryRepository struct {
	db *gorm.DB
}

// NewDiaryRepository 构造种植日记仓储。
func NewDiaryRepository(db *gorm.DB) DiaryRepository {
	return &diaryRepository{db: db}
}

func (r *diaryRepository) Create(d *model.DiaryEntry) error {
	return r.db.Create(d).Error
}

func (r *diaryRepository) Update(d *model.DiaryEntry) error {
	return r.db.Save(d).Error
}

func (r *diaryRepository) Delete(id uint) error {
	return r.db.Delete(&model.DiaryEntry{}, id).Error
}

func (r *diaryRepository) FindByID(id uint) (*model.DiaryEntry, error) {
	var d model.DiaryEntry
	if err := r.db.Preload("Plan").Preload("User").First(&d, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &d, nil
}

func (r *diaryRepository) List(pq util.PageQuery, userID, planID uint) ([]model.DiaryEntry, int64, error) {
	var diaries []model.DiaryEntry
	var total int64
	q := r.db.Model(&model.DiaryEntry{}).Preload("Plan").Preload("User")
	if userID > 0 {
		q = q.Where("user_id = ?", userID)
	}
	if planID > 0 {
		q = q.Where("plan_id = ?", planID)
	}
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := util.Paginate(q.Order("id DESC"), pq).Find(&diaries).Error; err != nil {
		return nil, 0, err
	}
	return diaries, total, nil
}

// ListVisible 非管理员可见日记：本人写的 + 本人作为 active 共管人地块下的全部日记。
// 共管关系终止后，历史日记仍随地块可见（共管人本人也仍能看到自己写过的内容）。
func (r *diaryRepository) ListVisible(pq util.PageQuery, viewerID, planID uint) ([]model.DiaryEntry, int64, error) {
	var diaries []model.DiaryEntry
	var total int64
	// 本人当前共管中的地块
	coPlotSub := r.db.Model(&model.PlotCaretaker{}).
		Select("plot_id").
		Where("caretaker_id = ? AND status = ?", viewerID, "active")
	// 可见计划：本人拥有的，或落在本人共管地块上的
	planSub := r.db.Model(&model.PlantingPlan{}).
		Select("id").
		Where("user_id = ? OR plot_id IN (?)", viewerID, coPlotSub)
	q := r.db.Model(&model.DiaryEntry{}).Preload("Plan").Preload("User").
		Where("user_id = ? OR plan_id IN (?)", viewerID, planSub)
	if planID > 0 {
		q = q.Where("plan_id = ?", planID)
	}
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := util.Paginate(q.Order("id DESC"), pq).Find(&diaries).Error; err != nil {
		return nil, 0, err
	}
	return diaries, total, nil
}

func (r *diaryRepository) IncrementLike(id uint) error {
	return r.db.Model(&model.DiaryEntry{}).Where("id = ?", id).UpdateColumn("like_count", gorm.Expr("like_count + 1")).Error
}

func (r *diaryRepository) CreateComment(c *model.DiaryComment) error {
	return r.db.Create(c).Error
}

func (r *diaryRepository) ListComments(diaryID uint) ([]model.DiaryComment, error) {
	var comments []model.DiaryComment
	if err := r.db.Preload("User").Where("diary_id = ?", diaryID).Order("id ASC").Find(&comments).Error; err != nil {
		return nil, err
	}
	return comments, nil
}

func (r *diaryRepository) CountByUser(userID uint) (int64, error) {
	var total int64
	if err := r.db.Model(&model.DiaryEntry{}).Where("user_id = ?", userID).Count(&total).Error; err != nil {
		return 0, err
	}
	return total, nil
}
