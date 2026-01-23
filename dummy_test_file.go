package main

import "fmt"

// RandomFunction is a test function for dummy exploration
func RandomFunction() {
	fmt.Println("This is a random test file")
}

// AnotherFunction demonstrates editing the file
func AnotherFunction(name string) string {
	return fmt.Sprintf("Hello, %s! This function was added during editing.", name)
}

func main() {
	RandomFunction()
	message := AnotherFunction("World")
	fmt.Println(message)
}
