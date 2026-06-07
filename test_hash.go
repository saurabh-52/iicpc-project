package main

import (
	"fmt"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	hash := "$2a$10$p4aFtWsjSlpYlaaHM/TGDuey6eOPpC80m/gBhvnb3XJdYh7oxBPby"
	passwords := []string{"123456", "password", "nipunk", "nipunk6", "qwerty"}
	for _, p := range passwords {
		err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(p))
		if err == nil {
			fmt.Println("Password is:", p)
			return
		}
	}
	fmt.Println("Password not found in common list")
}
