package main

import (
	"context"
	"fmt"
	"os"

	"github.com/georgysavva/scany/pgxscan"
	"github.com/jackc/pgx/v4"
)

func main() {
	fmt.Println("a")
	ctx := context.Background()

	conn, err := pgx.Connect(ctx, os.Getenv("DATABASE_URL"))

	if err != nil {
		panic(err)
	}
	defer conn.Close(ctx)
	fmt.Println("a")
	_, err = conn.Exec(ctx, "\\connect "+os.Getenv("DATABASE_NAME"))
	_, err = conn.Exec(ctx, `CREATE TABLE guilds(
		guildID bigint PRIMARY KEY
	)`)
	if err != nil {
		panic(err)
	}
	fmt.Println("a")

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
