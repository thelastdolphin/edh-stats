CREATE TABLE players
(
    "id"         serial  NOT NULL UNIQUE,
    "playerName" varchar NOT NULL UNIQUE,
    CONSTRAINT "players_pk" PRIMARY KEY ("id")
) WITH (
      OIDS= FALSE
    );