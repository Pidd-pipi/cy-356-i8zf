package router

import (
	"github.com/gin-gonic/gin"

	"github.com/communitygarden/server/internal/middleware"
)

// registerPlotCaretakers 地块共管路由（邀请/接受/移除均需登录）。
func (r *Router) registerPlotCaretakers(g *gin.RouterGroup) {
	auth := g.Group("")
	auth.Use(middleware.Auth(r.cfg, r.logger))
	{
		// 当前登录用户收到的待接受共管邀请
		auth.GET("/caretaker/invitations", r.caretakerHandler.MyInvitations)

		plots := auth.Group("/plots")
		{
			plots.POST("/:id/caretaker/invite", r.caretakerHandler.Invite)
			plots.POST("/:id/caretaker/accept", r.caretakerHandler.Accept)
			plots.DELETE("/:id/caretaker", r.caretakerHandler.Remove)
		}
	}
}
