package main

import "fmt"

func greet(name string) string {
	return fmt.Sprintf("Hello, %s!", name)
}

func main() {
	fmt.Println("This is a dummy test file")
	message := greet("iteratr")
	fmt.Println(message)
}
