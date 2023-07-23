CREATE TABLE decks
(
    "id"        serial  NOT NULL,
    "deck_name" varchar NOT NULL,
    "decklist"  TEXT    NOT NULL,
    CONSTRAINT "decks_pk" PRIMARY KEY ("id")
) WITH (
      OIDS= FALSE
    );