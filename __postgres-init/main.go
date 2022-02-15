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
	_, err = conn.Exec(ctx, "\\connect "+os.Getenv("DATABASE_NAME"))
	_, err = conn.Exec(ctx, "DROP TABLE guilds")
	_, err = conn.Exec(ctx, `CREATE TABLE guilds(
		guildID VARCHAR(18) PRIMARY KEY
	)`)
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
