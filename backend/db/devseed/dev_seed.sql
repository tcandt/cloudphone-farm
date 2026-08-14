-- Development Only Seed Data Structure (Executes ONLY when APP_ENV=development via cmd/devseed)

-- Seed Dev Organization
INSERT INTO organizations (organization_id, name, slug) VALUES
('org_pcp_enterprise_01', 'Enterprise Cloud Farm', 'enterprise-cloud-farm')
ON CONFLICT (organization_id) DO NOTHING;
