BEGIN;
ALTER TABLE "games"
    DROP CONSTRAINT "games_fk0";
ALTER TABLE "games"
    DROP CONSTRAINT "games_fk1";
ALTER TABLE "games"
    DROP CONSTRAINT "games_fk2";
ALTER TABLE "games"
    DROP CONSTRAINT "games_fk3";
ALTER TABLE "games"
    DROP CONSTRAINT "games_fk4";
ALTER TABLE "games"
    DROP CONSTRAINT "games_fk5";
ALTER TABLE "games"
    DROP CONSTRAINT "games_fk6";
ALTER TABLE "games"
    DROP CONSTRAINT "games_fk7";
COMMIT;