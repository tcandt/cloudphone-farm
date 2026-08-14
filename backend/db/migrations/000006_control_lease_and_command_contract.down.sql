-- Rollback Migration 000006

DROP TABLE IF EXISTS device_control_fences;

DELETE FROM role_permissions WHERE permission_id = 'device.control.acquire';
DELETE FROM permissions WHERE permission_id = 'device.control.acquire';
