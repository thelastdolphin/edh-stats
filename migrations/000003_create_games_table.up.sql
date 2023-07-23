CREATE TABLE games
(
    "id"              serial  NOT NULL,
    "date"            DATE    NOT NULL,
    "deck_id1"        integer NOT NULL,
    "deck_pilot1"     integer NOT NULL,
    "deck_id2"        integer NOT NULL,
    "deck_pilot2"     integer NOT NULL,
    "deck_id3"        integer,
    "deck_pilot3"     integer NOT NULL,
    "deck_id4"        integer,
    "deck_pilot4"     integer NOT NULL,
    "winner_deck_id"  integer NOT NULL,
    "winner_pilot_id" integer NOT NULL,
    "gametime"        TIME    NOT NULL,
    CONSTRAINT "games_pk" PRIMARY KEY ("id")
) WITH (
      OIDS= FALSE
    );