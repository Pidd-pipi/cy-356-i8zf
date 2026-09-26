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

// PlotCaretakerHandler 地块共管接口。
type PlotCaretakerHandler struct {
	caretakerService *service.PlotCaretakerService
	audit            middleware.AuditWriter
}

// NewPlotCaretakerHandler 构造地块共管接口。
func NewPlotCaretakerHandler(caretakerService *service.PlotCaretakerService, audit middleware.AuditWriter) *PlotCaretakerHandler {
	return &PlotCaretakerHandler{caretakerService: caretakerService, audit: audit}
}

// Invite 认养人邀请共管人。
func (h *PlotCaretakerHandler) Invite(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		util.Fail(c, http.StatusBadRequest, constants.CodeBadRequest, "路径参数 id 必须为正整数")
		return
	}
	var req dto.InviteCaretakerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		util.Fail(c, http.StatusBadRequest, constants.CodeValidationFailed, constants.ErrorText[constants.CodeValidationFailed]+": "+err.Error())
		return
	}
	claims, _ := util.GetClaims(c)
	caretaker, err := h.caretakerService.Invite(uint(id), claims.UserID, req.Username)
	if err != nil {
		util.FailWithAppError(c, err)
		return
	}
	_ = h.audit.Write(claims.UserID, claims.Username, claims.Role, "INVITE_CARETAKER", "plot_caretaker", strconv.FormatUint(uint64(id), 10),
		"邀请用户 "+req.Username+" 共管地块", c.ClientIP(), util.GetRequestID(c))
	util.OK(c, gin.H{"plot_id": caretaker.PlotID, "caretaker_id": caretaker.CaretakerID, "status": caretaker.Status, "message": constants.MsgCaretakerInviteOK})
}

// Accept 受邀人接受共管邀请。
func (h *PlotCaretakerHandler) Accept(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		util.Fail(c, http.StatusBadRequest, constants.CodeBadRequest, "路径参数 id 必须为正整数")
		return
	}
	claims, _ := util.GetClaims(c)
	caretaker, err := h.caretakerService.Accept(uint(id), claims.UserID)
	if err != nil {
		util.FailWithAppError(c, err)
		return
	}
	_ = h.audit.Write(claims.UserID, claims.Username, claims.Role, "ACCEPT_CARETAKER", "plot_caretaker", strconv.FormatUint(uint64(id), 10),
		"接受地块共管邀请", c.ClientIP(), util.GetRequestID(c))
	util.OK(c, gin.H{"plot_id": caretaker.PlotID, "caretaker_id": caretaker.CaretakerID, "status": caretaker.Status, "message": constants.MsgCaretakerAcceptOK})
}

// Remove 认养人移除共管人（或撤销待接受邀请）。
func (h *PlotCaretakerHandler) Remove(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		util.Fail(c, http.StatusBadRequest, constants.CodeBadRequest, "路径参数 id 必须为正整数")
		return
	}
	claims, _ := util.GetClaims(c)
	caretaker, err := h.caretakerService.Remove(uint(id), claims.UserID)
	if err != nil {
		util.FailWithAppError(c, err)
		return
	}
	_ = h.audit.Write(claims.UserID, claims.Username, claims.Role, "REMOVE_CARETAKER", "plot_caretaker", strconv.FormatUint(uint64(id), 10),
		"移除地块共管人 user_id="+strconv.FormatUint(uint64(caretaker.CaretakerID), 10), c.ClientIP(), util.GetRequestID(c))
	util.OK(c, gin.H{"plot_id": caretaker.PlotID, "caretaker_id": caretaker.CaretakerID, "status": caretaker.Status, "message": constants.MsgCaretakerRemoveOK})
}

// MyInvitations 当前用户收到的待接受邀请列表。
func (h *PlotCaretakerHandler) MyInvitations(c *gin.Context) {
	claims, _ := util.GetClaims(c)
	list, err := h.caretakerService.ListMyInvitations(claims.UserID)
	if err != nil {
		util.FailWithAppError(c, err)
		return
	}
	out := make([]*dto.CaretakerInvitationDTO, 0, len(list))
	for i := range list {
		out = append(out, dto.ToCaretakerInvitationDTO(&list[i]))
	}
	util.OK(c, out)
}
