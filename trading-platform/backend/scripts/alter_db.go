//go:build ignore

package main

import (
	"context"
	"fmt"
	"log"
	"github.com/jackc/pgx/v5"
)

func main() {
	pgURL := "postgres://user:password@127.0.0.1:5432/postgres?sslmode=disable"
	conn, err := pgx.Connect(context.Background(), pgURL)
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close(context.Background())

	q := `ALTER TABLE contest_final_scores ADD COLUMN avg_latency FLOAT8 NOT NULL DEFAULT 0, ADD COLUMN avg_throughput FLOAT8 NOT NULL DEFAULT 0, ADD COLUMN avg_correctness FLOAT8 NOT NULL DEFAULT 0, ADD COLUMN avg_p99 FLOAT8 NOT NULL DEFAULT 0, ADD COLUMN avg_tps FLOAT8 NOT NULL DEFAULT 0;`
	_, err = conn.Exec(context.Background(), q)
	if err != nil {
		log.Println(err)
	}
	fmt.Println("Done")
}
