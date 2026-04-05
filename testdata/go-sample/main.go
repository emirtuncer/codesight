package main

import "fmt"

func main() {
	u := NewUser("Alice", 30)
	fmt.Println(u.Greet())
}

func NewUser(name string, age int) *User {
	return &User{Name: name, Age: age}
}
