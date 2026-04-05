package main

import "fmt"

func main() {
	c := &Calculator{}
	result := c.Compute(1, 2)
	fmt.Println(Add(3, 4))
	fmt.Println(result)
}
