package main

import (
	"fmt"
	"net"
	"time"
)

func hello(i fmt.Stringer) {
	process(i)
}

func main() {
	var a time.Time
	var b net.IP
	process(a)
	hello(b)
}

func process(i fmt.Stringer) {
	i.String()
}
