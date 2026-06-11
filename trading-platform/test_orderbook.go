package main

import (
	"fmt"
	"net/http"
)

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})
	
	fmt.Println("Orderbook Engine Initialized...")
	fmt.Println("Listening on 0.0.0.0:8080 - Waiting for orders...")
	http.ListenAndServe(":8080", nil)
}
