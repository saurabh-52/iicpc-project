//go:build ignore

package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"github.com/Saurabh-52/trading-platform/internal/auth"
)

func main() {
	token, _ := auth.GenerateJWT("test-user", "test-user")
	req, _ := http.NewRequest("POST", "http://127.0.0.1:3001/contests/draft", strings.NewReader(`{"details": {"name": "test", "startTime": "2020-01-01T00:00:00Z"}}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	client := &http.Client{}
	res, _ := client.Do(req)
	body, _ := ioutil.ReadAll(res.Body)
	fmt.Printf("Status: %d\nBody: %s\n", res.StatusCode, string(body))
}
