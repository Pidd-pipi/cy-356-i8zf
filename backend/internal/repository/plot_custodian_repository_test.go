package repository

import (
	"testing"

	"gorm.io/gorm"

	"github.com/communitygarden/server/internal/constants"
	"github.com/communitygarden/server/internal/model"
)

func TestPlotCustodianRepository_OverviewAndCollaborativePlots(t *testing.T) {
	db := newTestDB(t)
	repo := NewPlotCustodianRepository(db)
	plotRepo := NewPlotRepository(db)

	owner := seedUser(t, db, "owner", "farmer")
	neighbor := seedUser(t, db, "neighbor", "citizen")

	ownerID := owner.ID
	plot1 := &model.Plot{Name: "P1", Code: "P1", Area: 10, SoilType: "loam", Sunlight: "full", Latitude: 31, Longitude: 121, Status: "adopted", AdopterID: &ownerID}
	plot2 := &model.Plot{Name: "P2", Code: "P2", Area: 10, SoilType: "loam", Sunlight: "full", Latitude: 31, Longitude: 121, Status: "available"}
	if err := db.Create(plot1).Error; err != nil {
		t.Fatalf("create plot1: %v", err)
	}
	if err := db.Create(plot2).Error; err != nil {
		t.Fatalf("create plot2: %v", err)
	}

	// plot1：一次已移除的旧邀请 + 一条 accepted（模拟移除后重新邀请并接受）
	old := &model.PlotCustodian{PlotID: plot1.ID, CustodianID: neighbor.ID, InviterID: owner.ID, Status: string(constants.CustodianRemoved)}
	if err := repo.Create(old); err != nil {
		t.Fatalf("create old: %v", err)
	}
	active := &model.PlotCustodian{PlotID: plot1.ID, CustodianID: neighbor.ID, InviterID: owner.ID, Status: string(constants.CustodianAccepted)}
	if err := repo.Create(active); err != nil {
		t.Fatalf("create active: %v", err)
	}

	// OverviewByPlotIDs：plot1 返回两条且最新在前
	overview, err := repo.OverviewByPlotIDs([]uint{plot1.ID, plot2.ID})
	if err != nil {
		t.Fatalf("OverviewByPlotIDs: %v", err)
	}
	if len(overview[plot1.ID]) != 2 || overview[plot1.ID][0].ID != active.ID {
		t.Fatalf("overview order/count invalid: %+v", overview[plot1.ID])
	}
	if _, ok := overview[plot2.ID]; ok {
		t.Fatalf("plot2 should have no custodian records")
	}

	// FindAcceptedByPlot
	got, err := repo.FindAcceptedByPlot(plot1.ID)
	if err != nil || got.ID != active.ID {
		t.Fatalf("FindAcceptedByPlot got=%v err=%v", got, err)
	}

	// ListCollaborativePlotIDs：邻居应包含 plot1；认养人 owner 也包含 plot1
	neighborPlots, err := repo.ListCollaborativePlotIDs(neighbor.ID)
	if err != nil {
		t.Fatalf("ListCollaborativePlotIDs neighbor: %v", err)
	}
	if !containsUint(neighborPlots, plot1.ID) {
		t.Fatalf("neighbor collaborative plots = %v, want %d", neighborPlots, plot1.ID)
	}
	ownerPlots, err := repo.ListCollaborativePlotIDs(owner.ID)
	if err != nil {
		t.Fatalf("ListCollaborativePlotIDs owner: %v", err)
	}
	if !containsUint(ownerPlots, plot1.ID) {
		t.Fatalf("owner adopted plots = %v, want %d", ownerPlots, plot1.ID)
	}

	// RevokeAllByPlotWithTx：释放时撤销有效记录，已移除的不变
	if err := db.Transaction(func(tx *gorm.DB) error {
		return repo.RevokeAllByPlotWithTx(tx, plot1.ID)
	}); err != nil {
		t.Fatalf("RevokeAllByPlotWithTx: %v", err)
	}
	after, err := repo.FindByID(active.ID)
	if err != nil || after.Status != string(constants.CustodianRemoved) || after.RemovedAt == nil {
		t.Fatalf("active record should be revoked: %+v err=%v", after, err)
	}
	_ = plotRepo
}

func containsUint(list []uint, v uint) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}
