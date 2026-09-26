package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/communitygarden/server/internal/constants"
	"github.com/communitygarden/server/internal/dto"
	"github.com/communitygarden/server/internal/middleware"
	"github.com/communitygarden/server/internal/service"
	"github.com/communitygarden/server/internal/util"
)

// PlotCustodianHandler 地块共管接口（邀请/接受/移除/历史）。
type PlotCustodianHandler struct {
	custodianService *service.PlotCustodianService
	audit            middleware.AuditWriter
}

// NewPlotCustodianHandler 构造地块共管接口。
func NewPlotCustodianHandler(custodianService *service.PlotCustodianService, audit middleware.AuditWriter) *PlotCustodianHandler {
	return &PlotCustodianHandler{custodianService: custodianService, audit: audit}
}

// Invite 认养人邀请共管人。
func (h *PlotCustodianHandler) Invite(c *gin.Context) {
	plotID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		util.Fail(c, http.StatusBadRequest, constants.CodeBadRequest, "路径参数 id 必须为正整数")
		return
	}
	var req dto.CreateCustodianInviteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		util.Fail(c, http.StatusBadRequest, constants.CodeValidationFailed, constants.ErrorText[constants.CodeValidationFailed]+": "+err.Error())
		return
	}
	claims, _ := util.GetClaims(c)
	rec, err := h.custodianService.Invite(uint(plotID), claims.UserID, req.Username)
	if err != nil {
		util.FailWithAppError(c, err)
		return
	}
	_ = h.audit.Write(claims.UserID, claims.Username, claims.Role, "INVITE_CUSTODIAN", "plot_custodian", strconv.FormatUint(uint64(rec.ID), 10),
		"邀请用户 "+req.Username+" 共管地块 id="+strconv.FormatUint(plotID, 10), c.ClientIP(), util.GetRequestID(c))
	util.OK(c, dto.ToPlotCustodianOutDTO(rec))
}

// Accept 被邀请人接受共管。
func (h *PlotCustodianHandler) Accept(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("custodianId"), 10, 64)
	if err != nil {
		util.Fail(c, http.StatusBadRequest, constants.CodeBadRequest, "路径参数 custodianId 必须为正整数")
		return
	}
	claims, _ := util.GetClaims(c)
	rec, err := h.custodianService.Accept(uint(id), claims.UserID)
	if err != nil {
		util.FailWithAppError(c, err)
		return
	}
	_ = h.audit.Write(claims.UserID, claims.Username, claims.Role, "ACCEPT_CUSTODIAN", "plot_custodian", strconv.FormatUint(id, 10),
		"接受地块 id="+strconv.FormatUint(uint64(rec.PlotID), 10)+" 的共管邀请", c.ClientIP(), util.GetRequestID(c))
	util.OK(c, dto.ToPlotCustodianOutDTO(rec))
}

// Remove 移除共管人/撤回或拒绝邀请。
func (h *PlotCustodianHandler) Remove(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("custodianId"), 10, 64)
	if err != nil {
		util.Fail(c, http.StatusBadRequest, constants.CodeBadRequest, "路径参数 custodianId 必须为正整数")
		return
	}
	claims, _ := util.GetClaims(c)
	rec, err := h.custodianService.Remove(uint(id), claims.UserID)
	if err != nil {
		util.FailWithAppError(c, err)
		return
	}
	_ = h.audit.Write(claims.UserID, claims.Username, claims.Role, "REMOVE_CUSTODIAN", "plot_custodian", strconv.FormatUint(id, 10),
		"移除地块 id="+strconv.FormatUint(uint64(rec.PlotID), 10)+" 的共管关系", c.ClientIP(), util.GetRequestID(c))
	util.OK(c, dto.ToPlotCustodianOutDTO(rec))
}

// History 查询地块共管历史（地块列表“共管”入口可见）。
func (h *PlotCustodianHandler) History(c *gin.Context) {
	plotID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		util.Fail(c, http.StatusBadRequest, constants.CodeBadRequest, "路径参数 id 必须为正整数")
		return
	}
	list, err := h.custodianService.ListByPlot(uint(plotID))
	if err != nil {
		util.FailWithAppError(c, err)
		return
	}
	out := make([]*dto.PlotCustodianOutDTO, 0, len(list))
	for i := range list {
		out = append(out, dto.ToPlotCustodianOutDTO(&list[i]))
	}
	util.OK(c, out)
}
