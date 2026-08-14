package device

import (
	"context"

	"github.com/tcandt/cloudphone-farm/backend/internal/domain"
	pgrepo "github.com/tcandt/cloudphone-farm/backend/internal/repository/postgres"
)

type DeviceService struct {
	repo *pgrepo.DeviceRepository
}

func NewDeviceService(repo *pgrepo.DeviceRepository) *DeviceService {
	return &DeviceService{repo: repo}
}

func (s *DeviceService) ListDevices(ctx context.Context, orgID string, params domain.DeviceListParams) (*domain.DeviceListResult, error) {
	return s.repo.ListDevices(ctx, orgID, params)
}

func (s *DeviceService) GetDeviceByID(ctx context.Context, orgID, deviceID string) (*domain.Device, error) {
	return s.repo.GetDeviceByID(ctx, orgID, deviceID)
}
