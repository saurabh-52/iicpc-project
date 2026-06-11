//go:build ignore

package main

import (
	"context"
	"fmt"
	"github.com/Saurabh-52/trading-platform/internal/store"
	"log"
)

func main() {
	pgURL := "postgres://user:password@127.0.0.1:5432/postgres?sslmode=disable"
	db, err := store.NewStore(context.Background(), pgURL)
	if err != nil {
		log.Fatal(err)
	}

	const q = `SELECT system_name, user_id, contest_id, judging_mode, total_score FROM submission_results WHERE contest_id = 'contest_mq8ey1y6_dz1vb'`
	rows, err := db.Pool().Query(context.Background(), q)
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	for rows.Next() {
		var s, u, c, j string
		var t float64
		rows.Scan(&s, &u, &c, &j, &t)
		fmt.Printf("SystemName: %s, UserID: %s, Contest: %s, Mode: %s, Score: %f\n", s, u, c, j, t)
	}
}
