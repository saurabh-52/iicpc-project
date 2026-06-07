package main

import (
	"fmt"
	"log"

	"github.com/Saurabh-52/trading-platform/internal/auth"
)

func main() {

	hash := "$2a$10$Y.ZgwfBqjoQddH5ywP.ftuPjzhOD0X0EmdhQgcSnZoyrI3Axe3RXm"
	testPass := "password123"
	fmt.Printf("Does 'password123' match DB hash? %v\n", auth.CheckPassword(hash, testPass))
}
