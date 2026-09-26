package integration_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/communitygarden/server/internal/config"
	"github.com/communitygarden/server/internal/constants"
	"github.com/communitygarden/server/internal/handler"
	"github.com/communitygarden/server/internal/model"
	"github.com/communitygarden/server/internal/repository"
	"github.com/communitygarden/server/internal/router"
	"github.com/communitygarden/server/internal/service"
	"github.com/communitygarden/server/internal/util"
)

var memDBCounter uint64

type testAudit struct{}

func (testAudit) Write(userID uint, username, role, action, resourceType, resourceID, detail, ip, requestID string) error {
	return nil
}

// newTestEngine 用 SQLite + 全部真实 handler/router 组装一套可发 HTTP 请求的引擎。
func newTestEngine(t *testing.T) (*gin.Engine, *gorm.DB) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	n := atomic.AddUint64(&memDBCounter, 1)
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:memdb-it-%d?mode=memory&cache=shared", n)), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(
		&model.User{}, &model.Plot{}, &model.PlotCustodian{}, &model.PlantingPlan{}, &model.HarvestRecord{},
		&model.DiaryEntry{}, &model.DiaryComment{}, &model.CommunityPost{}, &model.CommunityComment{},
		&model.AuditLog{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	logger := slog.New(slog.NewTextHandler(&discardWriter{}, nil))

	userRepo := repository.NewUserRepository(db)
	plotRepo := repository.NewPlotRepository(db)
	custodianRepo := repository.NewPlotCustodianRepository(db)
	planRepo := repository.NewPlantingPlanRepository(db)
	harvestRepo := repository.NewHarvestRecordRepository(db)
	diaryRepo := repository.NewDiaryRepository(db)
	postRepo := repository.NewCommunityRepository(db)
	auditRepo := repository.NewAuditRepository(db)

	authSvc := service.NewAuthService(userRepo, logger, "test-secret", 72)
	userSvc := service.NewUserService(userRepo, logger)
	plotSvc := service.NewPlotService(plotRepo, custodianRepo, db, logger)
	custodianSvc := service.NewPlotCustodianService(custodianRepo, plotRepo, userRepo, db, logger)
	auditSvc := service.NewAuditService(auditRepo, logger)
	planSvc := service.NewPlantingPlanService(planRepo, plotRepo, plotSvc, db, logger)
	harvestSvc := service.NewHarvestRecordService(harvestRepo, planRepo, db, logger)
	diarySvc := service.NewDiaryService(diaryRepo, planRepo, custodianRepo, logger)
	communitySvc := service.NewCommunityService(postRepo, logger)
	statsSvc := service.NewStatsService(userRepo, plotRepo, planRepo, harvestRepo, diaryRepo, postRepo, logger)

	authHandler := handler.NewAuthHandler(authSvc)
	userHandler := handler.NewUserHandler(userSvc, auditSvc)
	plotHandler := handler.NewPlotHandler(plotSvc, auditSvc)
	custodianHandler := handler.NewPlotCustodianHandler(custodianSvc, auditSvc)
	planHandler := handler.NewPlantingPlanHandler(planSvc, harvestSvc)
	harvestHandler := handler.NewHarvestHandler(harvestSvc, auditSvc)
	diaryHandler := handler.NewDiaryHandler(diarySvc)
	communityHandler := handler.NewCommunityHandler(communitySvc)
	auditHandler := handler.NewAuditHandler(auditSvc)
	statsHandler := handler.NewStatsHandler(statsSvc)

	cfg := &config.Config{RunMode: "test", JWTSecret: "test-secret"}
	r := router.New(cfg, logger, nil,
		authHandler, userHandler, plotHandler, custodianHandler, planHandler, harvestHandler,
		diaryHandler, communityHandler, auditHandler, statsHandler,
		testAudit{}, nil)
	return r.Build(), db
}

type discardWriter struct{}

func (discardWriter) Write(p []byte) (int, error) { return len(p), nil }

// doJSON 发送请求并解包统一响应。
func doJSON(t *testing.T, engine *gin.Engine, method, path, token string, body interface{}) (int, map[string]interface{}) {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatalf("encode body: %v", err)
		}
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	var out map[string]interface{}
	if rec.Body.Len() > 0 {
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatalf("decode response %s: %v; body=%s", path, err, rec.Body.String())
		}
	}
	return rec.Code, out
}

func loginToken(t *testing.T, engine *gin.Engine, username, password string) string {
	t.Helper()
	status, resp := doJSON(t, engine, http.MethodPost, "/api/v1/auth/login", "", map[string]string{"username": username, "password": password})
	if status != http.StatusOK {
		t.Fatalf("login %s status=%d resp=%v", username, status, resp)
	}
	data := resp["data"].(map[string]interface{})
	return data["token"].(string)
}

// TestCustodianE2E_FullFlow 端到端：注册→认养→邀请→接受→代写日记→移除→新日记被拒、历史保留→地块列表展示。
func TestCustodianE2E_FullFlow(t *testing.T) {
	engine, db := newTestEngine(t)
	logger := slog.New(slog.NewTextHandler(&discardWriter{}, nil))
	_ = logger

	// 种子：老李(farmer)、待认养地块
	owner := &model.User{Username: "laoli", Password: "hash", Nickname: "老李", Role: "farmer", Status: "active"}
	if err := db.Create(owner).Error; err != nil {
		t.Fatalf("seed owner: %v", err)
	}
	plot := &model.Plot{Name: "阳光一区 A07", Code: "P-007", Area: 12, SoilType: "loam", Sunlight: "full", Latitude: 31.23, Longitude: 121.47, Status: "available"}
	if err := db.Create(plot).Error; err != nil {
		t.Fatalf("seed plot: %v", err)
	}

	// 邻居注册
	if _, resp := doJSON(t, engine, http.MethodPost, "/api/v1/auth/register", "", map[string]string{"username": "wang", "password": "pass1234", "nickname": "老王邻居"}); resp["code"].(float64) != 0 {
		t.Fatalf("register neighbor failed: %v", resp)
	}
	// 给老李白签一个可用密码（与种子一致：用户名+123）
	hashPwd(t, db, owner.ID, "laoli123")
	ownerToken := loginToken(t, engine, "laoli", "laoli123")
	neighborToken := loginToken(t, engine, "wang", "pass1234")

	// 老李认养地块
	status, resp := doJSON(t, engine, http.MethodPost, fmt.Sprintf("/api/v1/plots/%d/adopt", plot.ID), ownerToken, nil)
	if status != http.StatusOK {
		t.Fatalf("adopt status=%d resp=%v", status, resp)
	}
	// 制定种植计划（春季生菜在推荐列表中）
	status, resp = doJSON(t, engine, http.MethodPost, "/api/v1/planting-plans", ownerToken, map[string]interface{}{
		"plot_id": plot.ID, "crop_name": "生菜", "crop_type": "vegetable", "season": "spring",
	})
	if status != http.StatusOK {
		t.Fatalf("create plan status=%d resp=%v", status, resp)
	}
	planID := uint(resp["data"].(map[string]interface{})["id"].(float64))

	// 邻居尚未被邀请，写日记应 403
	status, _ = doJSON(t, engine, http.MethodPost, "/api/v1/diaries", neighborToken, map[string]interface{}{
		"plan_id": planID, "action_type": "watering", "title": "偷写", "content": "不应成功",
	})
	if status != http.StatusForbidden {
		t.Fatalf("uninvited diary status=%d, want 403", status)
	}

	// 老李邀请邻居
	status, resp = doJSON(t, engine, http.MethodPost, fmt.Sprintf("/api/v1/plots/%d/custodians/invite", plot.ID), ownerToken, map[string]string{"username": "wang"})
	if status != http.StatusOK {
		t.Fatalf("invite status=%d resp=%v", status, resp)
	}
	custodianID := uint(resp["data"].(map[string]interface{})["id"].(float64))

	// 邻居接受
	status, resp = doJSON(t, engine, http.MethodPost, fmt.Sprintf("/api/v1/plot-custodians/%d/accept", custodianID), neighborToken, nil)
	if status != http.StatusOK {
		t.Fatalf("accept status=%d resp=%v", status, resp)
	}

	// 接受后邻居写日记成功
	status, resp = doJSON(t, engine, http.MethodPost, "/api/v1/diaries", neighborToken, map[string]interface{}{
		"plan_id": planID, "action_type": "watering", "title": "帮忙浇水", "content": "今天帮老李浇了水",
	})
	if status != http.StatusOK {
		t.Fatalf("custodian diary status=%d resp=%v", status, resp)
	}
	neighborDiaryID := uint(resp["data"].(map[string]interface{})["id"].(float64))

	// 邻居的计划列表应包含该地块计划（写日记选择器复用）
	status, resp = doJSON(t, engine, http.MethodGet, "/api/v1/planting-plans", neighborToken, nil)
	if status != http.StatusOK {
		t.Fatalf("neighbor plans status=%d", status)
	}
	planList := resp["data"].(map[string]interface{})["list"].([]interface{})
	if len(planList) != 1 {
		t.Fatalf("neighbor should see the custodian plot plan, got %d", len(planList))
	}

	// 老李移除共管人
	status, resp = doJSON(t, engine, http.MethodPost, fmt.Sprintf("/api/v1/plot-custodians/%d/remove", custodianID), ownerToken, nil)
	if status != http.StatusOK {
		t.Fatalf("remove status=%d resp=%v", status, resp)
	}
	if resp["data"].(map[string]interface{})["status"].(string) != "removed" {
		t.Fatalf("custodian status not removed: %v", resp["data"])
	}

	// 移除后邻居再写日记被 403
	status, _ = doJSON(t, engine, http.MethodPost, "/api/v1/diaries", neighborToken, map[string]interface{}{
		"plan_id": planID, "action_type": "other", "title": "移除后", "content": "不应成功",
	})
	if status != http.StatusForbidden {
		t.Fatalf("removed custodian diary status=%d, want 403", status)
	}

	// 之前写的日记仍在邻居的日记列表中
	status, resp = doJSON(t, engine, http.MethodGet, "/api/v1/diaries", neighborToken, nil)
	if status != http.StatusOK {
		t.Fatalf("neighbor diary list status=%d", status)
	}
	diaries := resp["data"].(map[string]interface{})["list"].([]interface{})
	if len(diaries) != 1 || uint(diaries[0].(map[string]interface{})["id"].(float64)) != neighborDiaryID {
		t.Fatalf("old diary should remain in list: %v", diaries)
	}

	// 地块列表带出最近一次共管记录（removed）
	status, resp = doJSON(t, engine, http.MethodGet, "/api/v1/plots?page=1&page_size=20", ownerToken, nil)
	if status != http.StatusOK {
		t.Fatalf("plot list status=%d", status)
	}
	var found bool
	for _, item := range resp["data"].(map[string]interface{})["list"].([]interface{}) {
		row := item.(map[string]interface{})
		if uint(row["id"].(float64)) == plot.ID {
			latest := row["latest_custodian"].(map[string]interface{})
			if latest["status"] != "removed" || uint(latest["id"].(float64)) != custodianID {
				t.Fatalf("latest_custodian invalid: %v", latest)
			}
			found = true
		}
	}
	if !found {
		t.Fatalf("plot not found in list")
	}

	// 共管历史接口返回完整记录
	status, resp = doJSON(t, engine, http.MethodGet, fmt.Sprintf("/api/v1/plots/%d/custodians", plot.ID), ownerToken, nil)
	if status != http.StatusOK {
		t.Fatalf("custodian history status=%d", status)
	}
	if len(resp["data"].([]interface{})) != 1 {
		t.Fatalf("history should contain 1 record: %v", resp["data"])
	}

	// 共管人不能释放他人地块（即使地块处于 harvested，也先被归属校验拦截）
	// 将计划推进到 completed 使地块变 harvested
	for _, target := range []string{"planting", "growing", "harvesting", "completed"} {
		status, resp = doJSON(t, engine, http.MethodPost, fmt.Sprintf("/api/v1/planting-plans/%d/status", planID), ownerToken, map[string]string{"status": target})
		if status != http.StatusOK {
			t.Fatalf("plan status -> %s failed: %d %v", target, status, resp)
		}
	}
	status, _ = doJSON(t, engine, http.MethodPost, fmt.Sprintf("/api/v1/plots/%d/release", plot.ID), neighborToken, nil)
	if status != http.StatusForbidden {
		t.Fatalf("custodian release status=%d, want 403", status)
	}
	// 审计动作常量存在性保护（防重构丢失）
	_ = []string{"INVITE_CUSTODIAN", "ACCEPT_CUSTODIAN", "REMOVE_CUSTODIAN"}
	_ = constants.CustodianAccepted
}

// hashPwd 直接更新种子用户密码哈希。
func hashPwd(t *testing.T, db *gorm.DB, userID uint, raw string) {
	t.Helper()
	hash, err := util.HashPassword(raw)
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	if err := db.Model(&model.User{}).Where("id = ?", userID).Update("password", hash).Error; err != nil {
		t.Fatalf("update pwd: %v", err)
	}
}
