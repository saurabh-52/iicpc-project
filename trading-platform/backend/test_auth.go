//go:build ignore

package main

import (
	"fmt"
	"github.com/Saurabh-52/trading-platform/internal/auth"
)

func main() {
	token, _ := auth.GenerateToken("test-user", "test-user")
	fmt.Println(token)
}
