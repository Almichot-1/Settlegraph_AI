-- Record and enforce transfer state transitions independently of journal effects.
CREATE TABLE sim_transfer_state_events (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
 transfer_id uuid NOT NULL REFERENCES sim_transfers(id),
 previous_state text,
 state text NOT NULL CHECK(state IN ('CREATED','ACCEPTED','SETTLEMENT_PENDING','SETTLED','FAILED')),
 occurred_at timestamptz NOT NULL,
 UNIQUE(transfer_id,state)
);
INSERT INTO sim_transfer_state_events(transfer_id,previous_state,state,occurred_at)
 SELECT id,NULL,state,updated_at FROM sim_transfers;
CREATE OR REPLACE FUNCTION record_sim_transfer_state() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF TG_OP='INSERT' THEN
   IF NEW.state<>'CREATED' THEN RAISE EXCEPTION 'transfer must start in CREATED'; END IF;
   INSERT INTO sim_transfer_state_events(transfer_id,previous_state,state,occurred_at)
     VALUES(NEW.id,NULL,NEW.state,NEW.updated_at);
   RETURN NEW;
 END IF;
 IF NEW.state=OLD.state THEN RETURN NEW; END IF;
 IF NOT ((OLD.state='CREATED' AND NEW.state IN ('ACCEPTED','FAILED'))
      OR (OLD.state='ACCEPTED' AND NEW.state IN ('SETTLEMENT_PENDING','FAILED'))
      OR (OLD.state='SETTLEMENT_PENDING' AND NEW.state IN ('SETTLED','FAILED'))) THEN
   RAISE EXCEPTION 'invalid transfer transition: % to %',OLD.state,NEW.state;
 END IF;
 INSERT INTO sim_transfer_state_events(transfer_id,previous_state,state,occurred_at)
   VALUES(NEW.id,OLD.state,NEW.state,NEW.updated_at);
 RETURN NEW;
END $$;
CREATE TRIGGER sim_transfer_created AFTER INSERT ON sim_transfers
 FOR EACH ROW EXECUTE FUNCTION record_sim_transfer_state();
CREATE TRIGGER sim_transfer_transition AFTER UPDATE OF state ON sim_transfers
 FOR EACH ROW EXECUTE FUNCTION record_sim_transfer_state();
