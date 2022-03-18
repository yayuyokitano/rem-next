package main

import (
	"context"
	"fmt"
	"os"

	"github.com/georgysavva/scany/pgxscan"
	"github.com/jackc/pgx/v4"
)

func main() {
	ctx := context.Background()

	conn, err := pgx.Connect(ctx, os.Getenv("DATABASE_URL"))

	if err != nil {
		panic(err)
	}
	defer conn.Close(ctx)
	_, err = conn.Exec(ctx, "\\c "+os.Getenv("DATABASE_NAME"))
	_, err = conn.Exec(ctx, `CREATE TABLE IF NOT EXISTS guilds(
		guildID VARCHAR(20) PRIMARY KEY
	)`)
	_, err = conn.Exec(ctx, `ALTER TABLE guilds ADD COLUMN IF NOT EXISTS cumulativeRoles BOOL NOT NULL DEFAULT FALSE`)
	if err != nil {
		panic(err)
	}

	_, err = conn.Exec(ctx, `CREATE TABLE IF NOT EXISTS rolerewards(
		guildID VARCHAR(20) NOT NULL,
		roleID VARCHAR(20) NOT NULL,
		level INTEGER NOT NULL,
		color INTEGER NOT NULL
	)`)
	_, err = conn.Exec(ctx, `ALTER TABLE rolerewards ADD COLUMN IF NOT EXISTS persistent BOOL NOT NULL DEFAULT FALSE`)
	if err != nil {
		panic(err)
	}

	_, err = conn.Exec(ctx, `CREATE TABLE IF NOT EXISTS channelblocklist(
		guildID VARCHAR(20) NOT NULL,
		channelID VARCHAR(20) PRIMARY KEY,
		xpgain bool NOT NULL DEFAULT FALSE
	)`)
	_, err = conn.Exec(ctx, `CREATE INDEX IF NOT EXISTS channelguildid ON channelblocklist(guildID)`)
	if err != nil {
		panic(err)
	}

	_, err = conn.Exec(ctx, `CREATE TABLE IF NOT EXISTS commands(
		commandID VARCHAR(20) PRIMARY KEY,
		guildID VARCHAR(20) NOT NULL,
		commandName VARCHAR(32) NOT NULL
	)`)
	_, err = conn.Exec(ctx, `CREATE INDEX IF NOT EXISTS command ON commands(guildID, commandName)`)
	if err != nil {
		panic(err)
	}

	_, err = conn.Exec(ctx, `CREATE TABLE IF NOT EXISTS guildXP(
		guildID VARCHAR(20) NOT NULL,
		userID VARCHAR(20) NOT NULL,
		nickname VARCHAR(32) NOT NULL,
		avatar VARCHAR(34) NOT NULL,
		xp BIGINT NOT NULL
	)`)
	_, err = conn.Exec(ctx, `CREATE UNIQUE INDEX IF NOT EXISTS xplookup ON guildXP(guildID, userID)`)
	if err != nil {
		panic(err)
	}

	var res []struct {
		Schemaname  string
		Tablename   string
		Tableowner  string
		Tablespace  *string
		Hasindexes  bool
		Hasrules    bool
		Hastriggers bool
		Rowsecurity bool
	}

	err = pgxscan.Select(ctx, conn, &res, `SELECT * FROM pg_catalog.pg_tables`)
	if err != nil {
		panic(err)
	}

	fmt.Print(res)

}
