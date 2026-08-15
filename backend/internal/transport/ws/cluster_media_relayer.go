package ws

import (
	"context"

	"github.com/tcandt/cloudphone-farm/backend/internal/agentws"
	"github.com/tcandt/cloudphone-farm/backend/internal/cluster"
	redispkg "github.com/tcandt/cloudphone-farm/backend/internal/repository/redis"
)

type ClusterMediaRelayer struct {
	mediaRepo *redispkg.MediaSessionRepository
	router    *cluster.ClusterRouter
}

func NewClusterMediaRelayer(mediaRepo *redispkg.MediaSessionRepository, router *cluster.ClusterRouter) *ClusterMediaRelayer {
	return &ClusterMediaRelayer{
		mediaRepo: mediaRepo,
		router:    router,
	}
}

func (r *ClusterMediaRelayer) RelayMediaSignalToBrowser(ctx context.Context, conn *agentws.Connection, sessionID string, data []byte) error {
	if r.mediaRepo == nil || r.router == nil {
		return agentws.ErrSessionNotFound
	}

	distSession, err := r.mediaRepo.GetMediaSession(ctx, sessionID)
	if err != nil || distSession == nil {
		return agentws.ErrSessionNotFound
	}

	// Verify connection fencing
	if conn.OrganizationID != distSession.OrganizationID ||
		conn.DeviceID != distSession.DeviceID ||
		conn.AgentID != distSession.AgentID ||
		conn.ConnectionID != distSession.ConnectionID ||
		conn.Generation != distSession.Generation {
		return agentws.ErrUnauthorizedMediaSession
	}

	return r.router.SendMediaSignalToBrowser(ctx, sessionID, distSession.BrowserNodeID, data)
}
