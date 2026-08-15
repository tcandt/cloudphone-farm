package ws

import (
	"context"
	"log/slog"

	"github.com/tcandt/cloudphone-farm/backend/internal/agentws"
	"github.com/tcandt/cloudphone-farm/backend/internal/cluster"
	redispkg "github.com/tcandt/cloudphone-farm/backend/internal/repository/redis"
)

type ClusterMediaRelayer struct {
	mediaRepo     *redispkg.MediaSessionRepository
	agentConnRepo *redispkg.AgentConnectionRepository
	router        *cluster.ClusterRouter
}

func NewClusterMediaRelayer(
	mediaRepo *redispkg.MediaSessionRepository,
	agentConnRepo *redispkg.AgentConnectionRepository,
	router *cluster.ClusterRouter,
) *ClusterMediaRelayer {
	return &ClusterMediaRelayer{
		mediaRepo:     mediaRepo,
		agentConnRepo: agentConnRepo,
		router:        router,
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

	// 1. Verify Media Session Snapshot Fencing
	if conn.OrganizationID != distSession.OrganizationID ||
		conn.DeviceID != distSession.DeviceID ||
		conn.AgentID != distSession.AgentID ||
		conn.ConnectionID != distSession.ConnectionID ||
		conn.Generation != distSession.Generation {
		return agentws.ErrUnauthorizedMediaSession
	}

	// 2. Global Current-Owner Fencing: Verify Agent connection is STILL the active Redis owner for this device
	if r.agentConnRepo != nil {
		currentOwner, err := r.agentConnRepo.GetOwner(ctx, conn.OrganizationID, conn.DeviceID)
		if err != nil || currentOwner == nil {
			slog.Warn("Device agent owner directory lookup failed during media relay. Rejecting.", "device_id", conn.DeviceID)
			return agentws.ErrUnauthorizedMediaSession
		}

		if currentOwner.AgentID != conn.AgentID ||
			currentOwner.ConnectionID != conn.ConnectionID ||
			currentOwner.Generation != conn.Generation {
			slog.Warn("Stale Agent connection attempted media relay post-takeover. Rejecting immediately.",
				"device_id", conn.DeviceID,
				"conn_id", conn.ConnectionID,
				"conn_gen", conn.Generation,
				"owner_conn_id", currentOwner.ConnectionID,
				"owner_gen", currentOwner.Generation,
			)
			return agentws.ErrUnauthorizedMediaSession
		}
	}

	return r.router.SendMediaSignalToBrowser(ctx, sessionID, distSession.BrowserNodeID, data)
}
