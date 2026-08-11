package main

import (
	"fmt"
	"time"
)

func main() {
	fmt.Println("OKF worker started")

	for {
		time.Sleep(30 * time.Second)
	}
}