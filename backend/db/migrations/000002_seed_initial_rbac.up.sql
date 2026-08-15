-- Seed System Immutable Permission Codes
INSERT INTO permissions (permission_code, category, description) VALUES
('device.read', 'device', 'View organization devices and metadata'),
('device.control.input', 'device', 'Acquire control lease and perform touch, swipe, text input gestures'),
('device.command.sensitive', 'device', 'Execute administrative commands such as reboot, lock, and APK installation'),
('device.stream.view', 'device', 'Access WebRTC stream session and live monitor feeds'),
('device.update', 'device', 'Modify device settings, proxy assignments, and group bindings'),
('organization.manage', 'organization', 'Manage organization team members, roles, and proxy profiles'),
('agent.enroll', 'agent', 'Generate one-time agent enrollment tokens')
ON CONFLICT (permission_code) DO NOTHING;

-- Seed System Default Roles (organization_id NULL indicates immutable system template roles)
INSERT INTO roles (role_id, organization_id, code, name, description) VALUES
('role_sys_owner', NULL, 'owner', 'Organization Owner', 'Full control over devices, billing, team, and organization settings'),
('role_sys_operator', NULL, 'operator', 'Device Operator', 'Can view streams, acquire control leases, and execute operational commands'),
('role_sys_viewer', NULL, 'viewer', 'Device Viewer', 'Read-only access to device lists and status feeds'),
('role_sys_admin', NULL, 'system_admin', 'Platform Administrator', 'System administrative rights')
ON CONFLICT (role_id) DO NOTHING;

-- Map Permissions to System Template Roles
-- Owner: All permissions
INSERT INTO role_permissions (role_id, permission_code) VALUES
('role_sys_owner', 'device.read'),
('role_sys_owner', 'device.control.input'),
('role_sys_owner', 'device.command.sensitive'),
('role_sys_owner', 'device.stream.view'),
('role_sys_owner', 'device.update'),
('role_sys_owner', 'organization.manage'),
('role_sys_owner', 'agent.enroll')
ON CONFLICT DO NOTHING;

-- Operator: device.read, device.control.input, device.stream.view, device.command.sensitive
INSERT INTO role_permissions (role_id, permission_code) VALUES
('role_sys_operator', 'device.read'),
('role_sys_operator', 'device.control.input'),
('role_sys_operator', 'device.stream.view'),
('role_sys_operator', 'device.command.sensitive')
ON CONFLICT DO NOTHING;

-- Viewer: device.read, device.stream.view
INSERT INTO role_permissions (role_id, permission_code) VALUES
('role_sys_viewer', 'device.read'),
('role_sys_viewer', 'device.stream.view')
ON CONFLICT DO NOTHING;
