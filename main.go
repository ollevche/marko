package main

import "github.com/ollevche/marko/cmd"

func main() {
	if err := cmd.RunREST(); err != nil {
		panic(err.Error())
	}
}
