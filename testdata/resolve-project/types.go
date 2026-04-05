package main

type Calculator struct {
	History []int
}

func (c *Calculator) Compute(a, b int) int {
	result := Add(a, b)
	c.History = append(c.History, result)
	return result
}
