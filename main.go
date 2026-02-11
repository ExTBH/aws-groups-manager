package main

import (
	"log"

	"aws-groups-manager/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		log.Fatal(err)
	}
}
