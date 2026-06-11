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

	teams, _ := db.GetContestRegistrations(context.Background(), "contest_mq8ey1y6_dz1vb")
	fmt.Printf("Teams: %v\n", teams)

	_, live, err := db.GetContestLeaderboard(context.Background(), "contest_mq8ey1y6_dz1vb", 100)
	if err != nil {
		fmt.Printf("GetContestLeaderboard err: %v\n", err)
	}
	for _, r := range live {
		fmt.Printf("Live Result: System=%s, User=%s, Score=%f\n", r.SystemName, r.Username, r.TotalScore)
	}
}
