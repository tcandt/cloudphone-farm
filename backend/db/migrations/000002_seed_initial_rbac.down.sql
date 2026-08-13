DELETE FROM role_permissions WHERE role_id IN ('role_sys_owner', 'role_sys_operator', 'role_sys_viewer', 'role_sys_admin');
DELETE FROM roles WHERE role_id IN ('role_sys_owner', 'role_sys_operator', 'role_sys_viewer', 'role_sys_admin');
DELETE FROM permissions WHERE permission_code IN ('device.read', 'device.control.input', 'device.command.sensitive', 'device.stream.view', 'device.update', 'organization.manage', 'agent.enroll');
