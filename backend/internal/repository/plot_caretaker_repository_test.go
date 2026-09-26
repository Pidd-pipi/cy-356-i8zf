package repository

import (
	"testing"

	"github.com/communitygarden/server/internal/model"
)

func TestPlotCaretakerRepository_OpenLifecycle(t *testing.T) {
	db := newTestDB(t)
	repo := NewPlotCaretakerRepository(db)
	plotRepo := NewPlotRepository(db)

	owner := seedUser(t, db, "owner", "farmer")
	neighbor := seedUser(t, db, "neighbor", "citizen")
	plot := &model.Plot{Name: "P-CT", Code: "P-CT", Area: 9, SoilType: "loam", Sunlight: "full", Latitude: 31, Longitude: 121, Status: "adopted", AdopterID: &owner.ID}
	if err := plotRepo.Create(plot); err != nil {
		t.Fatalf("create plot: %v", err)
	}

	c := &model.PlotCaretaker{PlotID: plot.ID, CaretakerID: neighbor.ID, InviterID: owner.ID, Status: "invited"}
	if err := repo.Create(c); err != nil {
		t.Fatalf("create caretaker: %v", err)
	}

	// 待接受邀请能被本人查到
	pending, err := repo.ListPendingByCaretaker(neighbor.ID)
	if err != nil || len(pending) != 1 {
		t.Fatalf("ListPendingByCaretaker len=%d err=%v", len(pending), err)
	}
	if pending[0].Plot == nil || pending[0].Plot.Code != "P-CT" {
		t.Errorf("pending invitation missing plot preload")
	}

	// invited 属于非终态，FindOpenByPlot 可查；active 尚不存在
	if open, err := repo.FindOpenByPlot(plot.ID); err != nil || open.Status != "invited" {
		t.Fatalf("FindOpenByPlot open=%v err=%v", open, err)
	}
	if _, err := repo.FindActiveByPlot(plot.ID); err == nil {
		t.Fatalf("FindActiveByPlot should be not found while invited")
	}

	// 接受后 active 可查
	c.Status = "active"
	if err := repo.Update(c); err != nil {
		t.Fatalf("update: %v", err)
	}
	if active, err := repo.FindActiveByPlot(plot.ID); err != nil || active.CaretakerID != neighbor.ID {
		t.Fatalf("FindActiveByPlot active=%v err=%v", active, err)
	}
	batch, err := repo.FindActiveByPlots([]uint{plot.ID, 9999})
	if err != nil || len(batch) != 1 || batch[0].Caretaker == nil {
		t.Fatalf("FindActiveByPlots len=%d err=%v", len(batch), err)
	}

	// 地块列表组装应带上当前共管记录
	gotPlot, err := plotRepo.FindByID(plot.ID)
	if err != nil {
		t.Fatalf("plot FindByID: %v", err)
	}
	if gotPlot.Caretaker == nil || gotPlot.Caretaker.CaretakerID != neighbor.ID {
		t.Fatalf("plot caretaker not assembled: %+v", gotPlot.Caretaker)
	}

	// 终态后不再出现在 open 查询中
	if err := repo.EndAllOpenByPlotWithTx(db, plot.ID); err != nil {
		t.Fatalf("end: %v", err)
	}
	if _, err := repo.FindOpenByPlot(plot.ID); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound after end, got err=%v", err)
	}
}
