package handler

import (
	"omniflow-go/internal/actor"
	"omniflow-go/internal/transport/http/middleware"

	"github.com/gin-gonic/gin"
)

func actorFromContext(ctx *gin.Context) actor.Actor {
	return middleware.ActorFromContext(ctx)
}
