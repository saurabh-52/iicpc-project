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

	const q = `UPDATE contest_final_scores SET avg_score = 81.1, avg_latency = 40.5, avg_throughput = 25.0, avg_correctness = 15.6, avg_p99 = 12.3, avg_tps = 45000.0, final_grade = 'A' WHERE system_name = 'mitul' AND contest_id = 'contest_mq8ey1y6_dz1vb'`
	_, err = conn.Exec(context.Background(), q)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Detailed scores restored!")
}
