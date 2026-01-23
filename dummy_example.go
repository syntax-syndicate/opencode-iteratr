package main

import "fmt"

// DummyGreeting returns a friendly greeting message
func DummyGreeting(name string) string {
	if name == "" {
		name = "World"
	}
	return fmt.Sprintf("Hello, %s!", name)
}

// DummyAdd adds two integers and returns the result
func DummyAdd(a, b int) int {
	return a + b
}
