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

// setupCustodian 构造共管测试环境：owner 认养 plot 并存在一个种植计划，neighbor 为被邀请邻居。
func setupCustodian(t *testing.T) (db *gorm.DB, plotSvc *PlotService, custodianSvc *PlotCustodianService, diarySvc *DiaryService, planSvc *PlantingPlanService, owner, neighbor *model.User, plot *model.Plot, plan *model.PlantingPlan) {
	t.Helper()
	gormDB := newTestServiceDB(t)
	plotRepo := repository.NewPlotRepository(gormDB)
	custodianRepo := repository.NewPlotCustodianRepository(gormDB)
	planRepo := repository.NewPlantingPlanRepository(gormDB)
	diaryRepo := repository.NewDiaryRepository(gormDB)
	userRepo := repository.NewUserRepository(gormDB)

	owner = newTestUser(t, gormDB, "laoli", "farmer")
	neighbor = newTestUser(t, gormDB, "neighbor", "citizen")
	plot = newTestPlot(t, gormDB, "P-CO", "available", nil)

	plotSvc = NewPlotService(plotRepo, custodianRepo, gormDB, testLogger())
	custodianSvc = NewPlotCustodianService(custodianRepo, plotRepo, userRepo, gormDB, testLogger())
	planSvc = NewPlantingPlanService(planRepo, plotRepo, plotSvc, gormDB, testLogger())
	diarySvc = NewDiaryService(diaryRepo, planRepo, custodianRepo, testLogger())

	if _, err := plotSvc.Adopt(plot.ID, owner.ID, "farmer", "laoli"); err != nil {
		t.Fatalf("adopt: %v", err)
	}
	var err error
	plan, err = planSvc.Create(&dto.CreatePlanRequest{PlotID: plot.ID, CropName: "生菜", CropType: "vegetable", Season: "spring"}, owner.ID)
	if err != nil {
		t.Fatalf("create plan: %v", err)
	}
	return gormDB, plotSvc, custodianSvc, diarySvc, planSvc, owner, neighbor, plot, plan
}

func pageQuery10() util.PageQuery {
	return util.PageQuery{Page: 1, PageSize: 10}
}

func TestPlotCustodian_InviteAcceptWriteDiary(t *testing.T) {
	db, plotSvc, custodianSvc, diarySvc, _, owner, neighbor, plot, plan := setupCustodian(t)

	// 1. 非认养人不能邀请
	if _, err := custodianSvc.Invite(plot.ID, neighbor.ID, "laoli"); err == nil {
		t.Fatalf("expected forbidden for non-adopter invite")
	}
	// 2. 认养人邀请注册用户
	rec, err := custodianSvc.Invite(plot.ID, owner.ID, neighbor.Username)
	if err != nil {
		t.Fatalf("Invite: %v", err)
	}
	if rec.Status != string(constants.CustodianPending) || rec.CustodianID != neighbor.ID {
		t.Fatalf("invite invalid: status=%s custodian=%d", rec.Status, rec.CustodianID)
	}
	// 3. 待接受期间不能重复邀请（同一人或他人）
	if _, err := custodianSvc.Invite(plot.ID, owner.ID, neighbor.Username); err == nil {
		t.Fatalf("expected duplicate pending invite conflict")
	}
	// 4. 其他人不能接受
	other := newTestUser(t, db, "other", "citizen")
	if _, err := custodianSvc.Accept(rec.ID, other.ID); err == nil {
		t.Fatalf("expected forbidden for non-invitee accept")
	}
	// 5. 接受前邻居不能写日记
	if _, err := diarySvc.Create(&dto.CreateDiaryRequest{PlanID: plan.ID, ActionType: "watering", Title: "帮忙浇水", Content: "pending"}, neighbor.ID); err == nil {
		t.Fatalf("pending custodian must not write diary")
	}
	// 6. 被邀请人接受
	accepted, err := custodianSvc.Accept(rec.ID, neighbor.ID)
	if err != nil {
		t.Fatalf("Accept: %v", err)
	}
	if accepted.Status != string(constants.CustodianAccepted) || accepted.AcceptedAt == nil {
		t.Fatalf("accept invalid: status=%s accepted_at=%v", accepted.Status, accepted.AcceptedAt)
	}
	// 7. accepted 共管人可以为同一地块的计划写日记
	d, err := diarySvc.Create(&dto.CreateDiaryRequest{PlanID: plan.ID, ActionType: "watering", Title: "帮忙浇水", Content: "土壤见干见湿"}, neighbor.ID)
	if err != nil {
		t.Fatalf("accepted custodian Create diary: %v", err)
	}
	if d.UserID != neighbor.ID {
		t.Fatalf("diary author = %d, want neighbor %d", d.UserID, neighbor.ID)
	}
	_ = plotSvc
}

func TestPlotCustodian_RemoveBlocksNewDiaryButKeepsHistory(t *testing.T) {
	_, _, custodianSvc, diarySvc, _, owner, neighbor, plot, plan := setupCustodian(t)

	rec, err := custodianSvc.Invite(plot.ID, owner.ID, neighbor.Username)
	if err != nil {
		t.Fatalf("Invite: %v", err)
	}
	if _, err := custodianSvc.Accept(rec.ID, neighbor.ID); err != nil {
		t.Fatalf("Accept: %v", err)
	}
	if _, err := diarySvc.Create(&dto.CreateDiaryRequest{PlanID: plan.ID, ActionType: "sowing", Title: "旧日记", Content: "共管期间"}, neighbor.ID); err != nil {
		t.Fatalf("custodian diary: %v", err)
	}

	// 原认养人移除共管人
	removed, err := custodianSvc.Remove(rec.ID, owner.ID)
	if err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if removed.Status != string(constants.CustodianRemoved) || removed.RemovedAt == nil {
		t.Fatalf("remove invalid: status=%s removed_at=%v", removed.Status, removed.RemovedAt)
	}
	// 移除后新日记写不进去
	if _, err := diarySvc.Create(&dto.CreateDiaryRequest{PlanID: plan.ID, ActionType: "other", Title: "新日记", Content: "不应成功"}, neighbor.ID); err == nil {
		t.Fatalf("removed custodian must not create new diary")
	}
	// 之前写的日记仍在列表中（邻居视角：本人写的历史日记保留）
	list, total, err := diarySvc.List(pageQuery10(), neighbor.ID, 0)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if total != 1 || len(list) != 1 || list[0].Title != "旧日记" {
		t.Fatalf("history diary should remain: total=%d len=%d", total, len(list))
	}
	// 移除后认养人可以重新邀请
	rec2, err := custodianSvc.Invite(plot.ID, owner.ID, neighbor.Username)
	if err != nil {
		t.Fatalf("re-invite after remove: %v", err)
	}
	if rec2.Status != string(constants.CustodianPending) {
		t.Fatalf("re-invite status = %s", rec2.Status)
	}
}

func TestPlotCustodian_ReleaseRevokesCustodian(t *testing.T) {
	_, plotSvc, custodianSvc, _, planSvc, owner, neighbor, plot, plan := setupCustodian(t)

	rec, err := custodianSvc.Invite(plot.ID, owner.ID, neighbor.Username)
	if err != nil {
		t.Fatalf("Invite: %v", err)
	}
	if _, err := custodianSvc.Accept(rec.ID, neighbor.ID); err != nil {
		t.Fatalf("Accept: %v", err)
	}
	// 走完整状态机到 completed 才能释放地块（adopted -> ... -> harvested -> release）
	for _, target := range []string{"planting", "growing", "harvesting", "completed"} {
		if _, err := planSvc.ChangeStatus(plan.ID, owner.ID, "farmer", target); err != nil {
			t.Fatalf("change status %s: %v", target, err)
		}
	}
	released, err := plotSvc.Release(plot.ID, owner.ID, "farmer")
	if err != nil {
		t.Fatalf("Release: %v", err)
	}
	if released.Status != string(constants.PlotStatusAvailable) {
		t.Fatalf("plot status = %s", released.Status)
	}
	history, err := custodianSvc.ListByPlot(plot.ID)
	if err != nil {
		t.Fatalf("ListByPlot: %v", err)
	}
	if len(history) != 1 || history[0].Status != string(constants.CustodianRemoved) {
		t.Fatalf("custodian should be revoked on release: %+v", history)
	}
}

func TestPlotCustodian_CannotInviteSelfOrAdoptOrRelease(t *testing.T) {
	db, plotSvc, custodianSvc, _, _, owner, _, plot, _ := setupCustodian(t)

	// 不能邀请自己
	if _, err := custodianSvc.Invite(plot.ID, owner.ID, owner.Username); err == nil {
		t.Fatalf("self invite must fail")
	}
	// 共管人不能认养：邻居对 available 地块可以认养，但对 owner 已认养地块认养会状态冲突
	neighbor2 := newTestUser(t, db, "neighbor2", "citizen")
	if _, err := plotSvc.Adopt(plot.ID, neighbor2.ID, "citizen", "neighbor2"); err == nil {
		t.Fatalf("non-available plot adopt must conflict")
	}
	// 共管人不能释放他人地块（harvested 状态校验前先做归属校验）
	if _, err := plotSvc.Release(plot.ID, neighbor2.ID, "citizen"); err == nil {
		t.Fatalf("non-owner release must be forbidden")
	}
}
