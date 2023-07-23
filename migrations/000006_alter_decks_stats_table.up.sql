BEGIN;
ALTER TABLE "decks_stats"
    ADD CONSTRAINT "decks_stats_fk0" FOREIGN KEY ("deck_id") REFERENCES "decks" ("id");
COMMIT;