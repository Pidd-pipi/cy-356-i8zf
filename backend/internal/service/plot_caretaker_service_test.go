package service

import (
	"testing"

	"gorm.io/gorm"

	"github.com/communitygarden/server/internal/constants"
	"github.com/communitygarden/server/internal/dto"
	"github.com/communitygarden/server/internal/model"
	"github.com/communitygarden/server/internal/repository"
	"github.com/communitygarden/server/internal/util"
)

// newCaretakerFixture 装配共管测试所需的用户、已认养地块与全部服务。
type caretakerFixture struct {
	db       *gorm.DB
	owner    *model.User
	neighbor *model.User
	stranger *model.User
	plot     *model.Plot
	plan     *model.PlantingPlan
	plotSvc  *PlotService
	careSvc  *PlotCaretakerService
	diarySvc *DiaryService
	planSvc  *PlantingPlanService
}

func newCaretakerFixture(t *testing.T) *caretakerFixture {
	t.Helper()
	db := newTestServiceDB(t)
	plotRepo := repository.NewPlotRepository(db)
	careRepo := repository.NewPlotCaretakerRepository(db)
	planRepo := repository.NewPlantingPlanRepository(db)
	diaryRepo := repository.NewDiaryRepository(db)

	plotSvc := NewPlotService(plotRepo, db, testLogger())
	careSvc := NewPlotCaretakerService(careRepo, repository.NewUserRepository(db), plotSvc, db, testLogger())
	plotSvc.SetCaretakerReleaser(careSvc.EndOpenByPlot)
	planSvc := NewPlantingPlanService(planRepo, plotRepo, plotSvc, db, testLogger())
	diarySvc := NewDiaryService(diaryRepo, planRepo, careRepo, testLogger())

	owner := newTestUser(t, db, "laoli", "farmer")
	neighbor := newTestUser(t, db, "neighbor", "citizen")
	stranger := newTestUser(t, db, "stranger", "citizen")
	plot := newTestPlot(t, db, "P-CARE", "available", nil)
	if _, err := plotSvc.Adopt(plot.ID, owner.ID, "farmer", "laoli"); err != nil {
		t.Fatalf("adopt: %v", err)
	}
	plan, err := planSvc.Create(&dto.CreatePlanRequest{
		PlotID: plot.ID, CropName: "生菜", CropType: "vegetable", Season: "spring",
	}, owner.ID)
	if err != nil {
		t.Fatalf("create plan: %v", err)
	}
	return &caretakerFixture{
		db:    db,
		owner: owner, neighbor: neighbor, stranger: stranger,
		plot: plot, plan: plan, plotSvc: plotSvc, careSvc: careSvc,
		diarySvc: diarySvc, planSvc: planSvc,
	}
}

func TestPlotCaretakerService_Invite(t *testing.T) {
	f := newCaretakerFixture(t)

	tests := []struct {
		name      string
		inviterID uint
		username  string
		wantErr   bool
	}{
		{name: "owner invites registered neighbor", inviterID: f.owner.ID, username: "neighbor", wantErr: false},
		{name: "duplicate invite conflicts", inviterID: f.owner.ID, username: "neighbor", wantErr: true},
		{name: "stranger cannot invite", inviterID: f.stranger.ID, username: "stranger", wantErr: true},
		{name: "owner cannot invite self", inviterID: f.owner.ID, username: "laoli", wantErr: true},
		{name: "invite unknown user", inviterID: f.owner.ID, username: "ghost", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := f.careSvc.Invite(f.plot.ID, tt.inviterID, tt.username)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("Invite: %v", err)
			}
			if got.Status != string(constants.CaretakerInvited) || got.CaretakerID != f.neighbor.ID {
				t.Errorf("invite result invalid: %+v", got)
			}
		})
	}
}

func TestPlotCaretakerService_Accept(t *testing.T) {
	f := newCaretakerFixture(t)
	if _, err := f.careSvc.Invite(f.plot.ID, f.owner.ID, "neighbor"); err != nil {
		t.Fatalf("invite: %v", err)
	}

	// 非受邀人不能接受
	if _, err := f.careSvc.Accept(f.plot.ID, f.stranger.ID); err == nil {
		t.Fatalf("stranger accepted invitation")
	}
	// 受邀人接受
	got, err := f.careSvc.Accept(f.plot.ID, f.neighbor.ID)
	if err != nil {
		t.Fatalf("Accept: %v", err)
	}
	if got.Status != string(constants.CaretakerActive) || got.AcceptedAt == nil {
		t.Errorf("accept result invalid: status=%s accepted_at=%v", got.Status, got.AcceptedAt)
	}
	// 重复接受冲突
	if _, err := f.careSvc.Accept(f.plot.ID, f.neighbor.ID); err == nil {
		t.Fatalf("duplicate accept should conflict")
	}
}

func TestPlotCaretakerService_DiaryWriteAndRemove(t *testing.T) {
	f := newCaretakerFixture(t)

	// 未接受前，邻居写不进日记
	if _, err := f.diarySvc.Create(&dto.CreateDiaryRequest{
		PlanID: f.plan.ID, ActionType: "watering", Title: "帮浇水", Content: "被拒绝",
	}, f.neighbor.ID); err == nil {
		t.Fatalf("pending caretaker should not write diary")
	}

	if _, err := f.careSvc.Invite(f.plot.ID, f.owner.ID, "neighbor"); err != nil {
		t.Fatalf("invite: %v", err)
	}
	if _, err := f.careSvc.Accept(f.plot.ID, f.neighbor.ID); err != nil {
		t.Fatalf("accept: %v", err)
	}

	// 共管中：邻居可以给同一地块下的计划写日记
	d, err := f.diarySvc.Create(&dto.CreateDiaryRequest{
		PlanID: f.plan.ID, ActionType: "watering", Title: "帮老李浇水", Content: "土壤见干见湿",
	}, f.neighbor.ID)
	if err != nil {
		t.Fatalf("active caretaker create diary: %v", err)
	}
	// 陌生人仍然不能写
	if _, err := f.diarySvc.Create(&dto.CreateDiaryRequest{
		PlanID: f.plan.ID, ActionType: "other", Title: "x", Content: "x",
	}, f.stranger.ID); err == nil {
		t.Fatalf("stranger should not write diary")
	}

	// 认养人移除共管人（共管人自己不能移除）
	if _, err := f.careSvc.Remove(f.plot.ID, f.neighbor.ID); err == nil {
		t.Fatalf("caretaker must not remove itself")
	}
	if _, err := f.careSvc.Remove(f.plot.ID, f.owner.ID); err != nil {
		t.Fatalf("owner remove: %v", err)
	}

	// 移除后新日记写不进去
	if _, err := f.diarySvc.Create(&dto.CreateDiaryRequest{
		PlanID: f.plan.ID, ActionType: "other", Title: "再写一篇", Content: "应被拒绝",
	}, f.neighbor.ID); err == nil {
		t.Fatalf("removed caretaker must not write new diary")
	}
	// 之前写的日记仍在列表中（本人视角）
	list, total, err := f.diarySvc.List(util.PageQuery{Page: 1, PageSize: 10}, f.neighbor.ID, false, 0)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total != 1 || list[0].ID != d.ID {
		t.Errorf("history diary should remain: total=%d", total)
	}
	// 移除后可以重新邀请新人
	if _, err := f.careSvc.Invite(f.plot.ID, f.owner.ID, "stranger"); err != nil {
		t.Fatalf("re-invite after remove: %v", err)
	}
}

func TestPlotCaretakerService_ReleaseEndsCaretaker(t *testing.T) {
	f := newCaretakerFixture(t)
	if _, err := f.careSvc.Invite(f.plot.ID, f.owner.ID, "neighbor"); err != nil {
		t.Fatalf("invite: %v", err)
	}
	if _, err := f.careSvc.Accept(f.plot.ID, f.neighbor.ID); err != nil {
		t.Fatalf("accept: %v", err)
	}

	// 释放要求 harvested 状态
	if err := f.db.Model(&model.Plot{}).Where("id = ?", f.plot.ID).Update("status", "harvested").Error; err != nil {
		t.Fatalf("mark harvested: %v", err)
	}
	if _, err := f.plotSvc.Release(f.plot.ID, f.owner.ID, "farmer"); err != nil {
		t.Fatalf("release: %v", err)
	}
	// 共管关系随释放终止，邻居写不进新日记
	if _, err := f.diarySvc.Create(&dto.CreateDiaryRequest{
		PlanID: f.plan.ID, ActionType: "other", Title: "释放后", Content: "应被拒绝",
	}, f.neighbor.ID); err == nil {
		t.Fatalf("caretaker diary should be blocked after plot release")
	}
}
