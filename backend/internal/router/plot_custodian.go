package router

import (
	"github.com/gin-gonic/gin"

	"github.com/communitygarden/server/internal/middleware"
)

// registerPlotCustodians 地块共管路由（全部需要登录；细粒度权限在 service 层校验）。
func (r *Router) registerPlotCustodians(g *gin.RouterGroup) {
	// 认养人视角：发起邀请、查看某地块共管历史
	plots := g.Group("/plots")
	plots.Use(middleware.Auth(r.cfg, r.logger))
	{
		plots.POST("/:id/custodians/invite", r.custodianHandler.Invite)
		plots.GET("/:id/custodians", r.custodianHandler.History)
	}

	// 共管人视角：接受邀请、退出/拒绝；认养人视角：移除共管人
	custodians := g.Group("/plot-custodians")
	custodians.Use(middleware.Auth(r.cfg, r.logger))
	{
		custodians.POST("/:custodianId/accept", r.custodianHandler.Accept)
		custodians.POST("/:custodianId/remove", r.custodianHandler.Remove)
	}
}
