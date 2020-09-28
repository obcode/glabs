package main

import (
	"github.com/obcode/glabs/cmd"
)

func main() {
	err := cmd.Execute()
	if err != nil {
		panic(err)
	}
}
