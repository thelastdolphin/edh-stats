CREATE TABLE decks_stats
(
    "deck_id"      serial  NOT NULL,
    "games_count"  integer NOT NULL,
    "wins_count"   integer NOT NULL,
    "losses_count" integer NOT NULL,
    "winrate"      integer NOT NULL,
    CONSTRAINT "decks_stats_pk" PRIMARY KEY ("deck_id")
) WITH (
      OIDS= FALSE
    );