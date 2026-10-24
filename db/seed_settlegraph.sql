BEGIN;
INSERT INTO organizations(id,name,mode) VALUES ('11111111-1111-4111-8111-111111111111','SettleGraph Synthetic Demo','DEMO') ON CONFLICT (id) DO NOTHING;
INSERT INTO app_users(id,auth_subject,display_name) VALUES ('22222222-2222-4222-8222-222222222222','demo-owner','Demo Owner') ON CONFLICT (id) DO NOTHING;
INSERT INTO memberships(organization_id,user_id,role) VALUES ('11111111-1111-4111-8111-111111111111','22222222-2222-4222-8222-222222222222','OWNER') ON CONFLICT (organization_id,user_id) DO NOTHING;
INSERT INTO provider_sources(organization_id,id,label,source_type) VALUES
 ('11111111-1111-4111-8111-111111111111','BANK_A','Fictional Bank A','BANK'),
 ('11111111-1111-4111-8111-111111111111','BANK_B','Fictional Bank B','BANK')
 ON CONFLICT (organization_id,id) DO NOTHING;
INSERT INTO business_accounts(organization_id,id,provider_id,account_label,account_type) VALUES
 ('11111111-1111-4111-8111-111111111111','aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaa1','BANK_A','Synthetic customer at Bank A','CUSTOMER'),
 ('11111111-1111-4111-8111-111111111111','bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbb1','BANK_B','Synthetic merchant at Bank B','MERCHANT')
 ON CONFLICT (organization_id,id) DO NOTHING;
COMMIT;
