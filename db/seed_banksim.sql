BEGIN;
INSERT INTO sim_institutions(id,name,kind) VALUES
 ('BANK_A','Fictional Bank A','BANK'),('BANK_B','Fictional Bank B','BANK')
 ON CONFLICT (id) DO NOTHING;
INSERT INTO sim_accounts(id,institution_id,owner_label,account_type,balance_minor) VALUES
 ('aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaa1','BANK_A','Synthetic payer','CUSTOMER',0),
 ('aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaa2','BANK_A','Synthetic clearing','CLEARING',0),
 ('bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbb1','BANK_B','Synthetic merchant','MERCHANT',0),
 ('aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaa3','BANK_A','Synthetic opening equity','EQUITY',0)
 ON CONFLICT (id) DO NOTHING;
INSERT INTO sim_journals(id,action,status,currency,occurred_at)
 VALUES ('cccccccc-cccc-4ccc-8ccc-ccccccccccc1','OPENING_BALANCE','DRAFT','ETB','2026-10-24T09:00:00Z')
 ON CONFLICT (id) DO NOTHING;
INSERT INTO sim_entries(id,journal_id,account_id,signed_minor)
 SELECT * FROM (VALUES
 ('dddddddd-dddd-4ddd-8ddd-ddddddddddd1'::uuid,'cccccccc-cccc-4ccc-8ccc-ccccccccccc1'::uuid,'aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaa1'::uuid,100000000),
 ('dddddddd-dddd-4ddd-8ddd-ddddddddddd2','cccccccc-cccc-4ccc-8ccc-ccccccccccc1','aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaa3',-100000000)
 ) AS entries(id,journal_id,account_id,signed_minor)
 WHERE NOT EXISTS (SELECT 1 FROM sim_journals WHERE id='cccccccc-cccc-4ccc-8ccc-ccccccccccc1' AND status='POSTED')
 ON CONFLICT (id) DO NOTHING;
UPDATE sim_accounts SET balance_minor=100000000 WHERE id='aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaa1' AND balance_minor=0;
UPDATE sim_accounts SET balance_minor=-100000000 WHERE id='aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaa3' AND balance_minor=0;
UPDATE sim_journals SET status='POSTED' WHERE id='cccccccc-cccc-4ccc-8ccc-ccccccccccc1' AND status='DRAFT';
COMMIT;
