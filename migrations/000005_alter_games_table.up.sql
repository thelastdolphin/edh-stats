BEGIN;

ALTER TABLE "games"
    ADD CONSTRAINT "games_fk0" FOREIGN KEY ("deck_id1") REFERENCES "decks" ("id");
ALTER TABLE "games"
    ADD CONSTRAINT "games_fk1" FOREIGN KEY ("deck_pilot1") REFERENCES "players" ("id");
ALTER TABLE "games"
    ADD CONSTRAINT "games_fk2" FOREIGN KEY ("deck_id2") REFERENCES "decks" ("id");
ALTER TABLE "games"
    ADD CONSTRAINT "games_fk3" FOREIGN KEY ("deck_pilot2") REFERENCES "players" ("id");
ALTER TABLE "games"
    ADD CONSTRAINT "games_fk4" FOREIGN KEY ("deck_id3") REFERENCES "decks" ("id");
ALTER TABLE "games"
    ADD CONSTRAINT "games_fk5" FOREIGN KEY ("deck_pilot3") REFERENCES "players" ("id");
ALTER TABLE "games"
    ADD CONSTRAINT "games_fk6" FOREIGN KEY ("deck_id4") REFERENCES "decks" ("id");
ALTER TABLE "games"
    ADD CONSTRAINT "games_fk7" FOREIGN KEY ("deck_pilot4") REFERENCES "players"("id");

COMMIT;