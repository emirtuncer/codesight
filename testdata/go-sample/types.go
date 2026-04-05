package main

import "fmt"

// User represents a person.
type User struct {
	Name string
	Age  int
}

// Greet returns a greeting string.
func (u *User) Greet() string {
	return fmt.Sprintf("Hello, I'm %s", u.Name)
}

// Stringer is an interface for types that can convert to string.
type Stringer interface {
	String() string
}
