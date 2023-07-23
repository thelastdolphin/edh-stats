BEGIN;
ALTER TABLE "decks_stats"
    DROP CONSTRAINT "decks_stats_fk0";
COMMIT;