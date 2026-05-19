//go:build ignore
// +build ignore

package main

import (
	"fmt" // Package for formatted I/O
)

// main is the entry point of the program
func main() {
	// Declare and initialize variables
	var name string = "Go Programmer"
	age := 25 // Short variable declaration

	// Print output
	fmt.Println("Hello,", name)
	fmt.Printf("You are %d years old.\n", age)

	// Call a custom function
	sum := add(10, 20)
	fmt.Printf("The sum of 10 and 20 is: %d\n", sum)
}

// add takes two integers and returns their sum
func add(a int, b int) int {
	return a + b
}
