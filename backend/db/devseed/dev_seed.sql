-- Development Only Seed Data (Executes ONLY when APP_ENV=development)

-- Seed Dev Organization
INSERT INTO organizations (organization_id, name, slug) VALUES
('org_pcp_enterprise_01', 'Enterprise Cloud Farm', 'enterprise-cloud-farm')
ON CONFLICT (organization_id) DO NOTHING;

-- Seed Dev User (Minh Tuấn - Owner)
INSERT INTO users (user_id, email, password_hash, display_name) VALUES
('usr_owner_01', 'owner@phonecontrol.io', '$argon2id$v=19$m=65536,t=3,p=4$c29tZXNhbHQ$hashvalue', 'Minh Tuấn (Owner)')
ON CONFLICT (user_id) DO NOTHING;

-- Seed Dev Organization Membership
INSERT INTO organization_memberships (membership_id, organization_id, user_id, status) VALUES
('mem_owner_01', 'org_pcp_enterprise_01', 'usr_owner_01', 'active')
ON CONFLICT (membership_id) DO NOTHING;

-- Assign Owner Role to Dev Membership
INSERT INTO user_roles (membership_id, role_id) VALUES
('mem_owner_01', 'role_sys_owner')
ON CONFLICT DO NOTHING;
