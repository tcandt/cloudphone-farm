package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/tcandt/cloudphone-farm/backend/internal/domain"
)

type DeviceRepository struct {
	pool               *pgxpool.Pool
	onlineThresholdSec  int
	offlineThresholdSec int
}

func NewDeviceRepository(pool *pgxpool.Pool, onlineThresholdSec, offlineThresholdSec int) *DeviceRepository {
	if onlineThresholdSec <= 0 {
		onlineThresholdSec = 30
	}
	if offlineThresholdSec <= 0 {
		offlineThresholdSec = 90
	}
	return &DeviceRepository{
		pool:                pool,
		onlineThresholdSec:  onlineThresholdSec,
		offlineThresholdSec: offlineThresholdSec,
	}
}

// ListDevices returns tenant-scoped devices with latest heartbeat telemetry via a single LATERAL JOIN query
func (r *DeviceRepository) ListDevices(ctx context.Context, orgID string, params domain.DeviceListParams) (*domain.DeviceListResult, error) {
	if r.pool == nil {
		return nil, errors.New("postgres connection pool uninitialized")
	}

	page := params.Page
	if page < 1 {
		page = 1
	}

	limit := params.Limit
	if limit < 1 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}

	offset := (page - 1) * limit

	var whereClauses []string
	var args []interface{}
	argIdx := 1

	whereClauses = append(whereClauses, fmt.Sprintf("d.organization_id = $%d", argIdx))
	args = append(args, orgID)
	argIdx++

	if params.GroupID != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("d.group_id = $%d", argIdx))
		args = append(args, params.GroupID)
		argIdx++
	}

	if params.Status != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("d.status = $%d", argIdx))
		args = append(args, params.Status)
		argIdx++
	}

	if params.Search != "" {
		searchPattern := "%" + strings.TrimSpace(params.Search) + "%"
		whereClauses = append(whereClauses, fmt.Sprintf("(d.name ILIKE $%d OR d.serial_number ILIKE $%d OR d.model ILIKE $%d)", argIdx, argIdx, argIdx))
		args = append(args, searchPattern)
		argIdx++
	}

	whereStmt := strings.Join(whereClauses, " AND ")

	// 1. Count query
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM devices d WHERE %s", whereStmt)
	var total int
	if err := r.pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, fmt.Errorf("failed to count devices: %w", err)
	}

	// 2. Data query with LATERAL JOIN on latest heartbeat
	dataQuery := fmt.Sprintf(`
		SELECT
			d.device_id,
			d.organization_id,
			d.group_id,
			d.name,
			d.serial_number,
			d.model,
			d.platform_version,
			d.status,
			d.capabilities,
			d.created_at,
			d.updated_at,
			hb.cpu_usage,
			hb.memory_usage,
			hb.battery_level,
			hb.temperature_c,
			hb.network_type,
			hb.received_at
		FROM devices d
		LEFT JOIN LATERAL (
			SELECT cpu_usage, memory_usage, battery_level, temperature_c, network_type, received_at
			FROM device_heartbeats
			WHERE organization_id = d.organization_id AND device_id = d.device_id
			ORDER BY received_at DESC
			LIMIT 1
		) hb ON TRUE
		WHERE %s
		ORDER BY d.created_at DESC
		LIMIT $%d OFFSET $%d
	`, whereStmt, argIdx, argIdx+1)

	queryArgs := append(args, limit, offset)

	rows, err := r.pool.Query(ctx, dataQuery, queryArgs...)
	if err != nil {
		return nil, fmt.Errorf("failed to query devices list: %w", err)
	}
	defer rows.Close()

	var items []domain.Device
	for rows.Next() {
		var dev domain.Device
		var rawCaps []byte
		var cpu, mem, temp *float64
		var battery *int
		var netType *string
		var lastSeen *time.Time

		err := rows.Scan(
			&dev.DeviceID,
			&dev.OrganizationID,
			&dev.GroupID,
			&dev.Name,
			&dev.SerialNumber,
			&dev.Model,
			&dev.PlatformVersion,
			&dev.Status,
			&rawCaps,
			&dev.CreatedAt,
			&dev.UpdatedAt,
			&cpu,
			&mem,
			&battery,
			&temp,
			&netType,
			&lastSeen,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan device row: %w", err)
		}

		dev.Capabilities = domain.ParseCapabilitiesJSON(rawCaps)
		dev.LastSeenAt = lastSeen

		if lastSeen != nil {
			dev.Status = domain.DeriveRealtimeStatus(dev.Status, lastSeen, r.onlineThresholdSec, r.offlineThresholdSec)
			t := &domain.DeviceTelemetry{
				UpdatedAt: *lastSeen,
			}
			if battery != nil {
				t.Battery = *battery
			}
			if netType != nil {
				t.Network = *netType
			}
			if cpu != nil {
				t.CPUUsage = *cpu
			}
			if mem != nil {
				t.RAMUsage = *mem
			}
			if temp != nil {
				t.TemperatureC = *temp
			}
			dev.Telemetry = t
		}

		items = append(items, dev)
	}

	if items == nil {
		items = []domain.Device{}
	}

	return &domain.DeviceListResult{
		Items: items,
		Page:  page,
		Limit: limit,
		Total: total,
	}, nil
}

// GetDeviceByID returns single device for tenant, ensuring strict organization_id AND device_id filtering
func (r *DeviceRepository) GetDeviceByID(ctx context.Context, orgID, deviceID string) (*domain.Device, error) {
	if r.pool == nil {
		return nil, errors.New("postgres connection pool uninitialized")
	}

	query := `
		SELECT
			d.device_id,
			d.organization_id,
			d.group_id,
			d.name,
			d.serial_number,
			d.model,
			d.platform_version,
			d.status,
			d.capabilities,
			d.created_at,
			d.updated_at,
			hb.cpu_usage,
			hb.memory_usage,
			hb.battery_level,
			hb.temperature_c,
			hb.network_type,
			hb.received_at
		FROM devices d
		LEFT JOIN LATERAL (
			SELECT cpu_usage, memory_usage, battery_level, temperature_c, network_type, received_at
			FROM device_heartbeats
			WHERE organization_id = d.organization_id AND device_id = d.device_id
			ORDER BY received_at DESC
			LIMIT 1
		) hb ON TRUE
		WHERE d.organization_id = $1 AND d.device_id = $2
	`

	var dev domain.Device
	var rawCaps []byte
	var cpu, mem, temp *float64
	var battery *int
	var netType *string
	var lastSeen *time.Time

	err := r.pool.QueryRow(ctx, query, orgID, deviceID).Scan(
		&dev.DeviceID,
		&dev.OrganizationID,
		&dev.GroupID,
		&dev.Name,
		&dev.SerialNumber,
		&dev.Model,
		&dev.PlatformVersion,
		&dev.Status,
		&rawCaps,
		&dev.CreatedAt,
		&dev.UpdatedAt,
		&cpu,
		&mem,
		&battery,
		&temp,
		&netType,
		&lastSeen,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrDeviceNotFound
		}
		return nil, fmt.Errorf("failed to fetch device by ID: %w", err)
	}

	dev.Capabilities = domain.ParseCapabilitiesJSON(rawCaps)
	dev.LastSeenAt = lastSeen

	if lastSeen != nil {
		dev.Status = domain.DeriveRealtimeStatus(dev.Status, lastSeen, r.onlineThresholdSec, r.offlineThresholdSec)
		t := &domain.DeviceTelemetry{
			UpdatedAt: *lastSeen,
		}
		if battery != nil {
			t.Battery = *battery
		}
		if netType != nil {
			t.Network = *netType
		}
		if cpu != nil {
			t.CPUUsage = *cpu
		}
		if mem != nil {
			t.RAMUsage = *mem
		}
		if temp != nil {
			t.TemperatureC = *temp
		}
		dev.Telemetry = t
	}

	return &dev, nil
}
