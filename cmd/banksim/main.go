package main

import (
	"log"

	"github.com/settlegraph/settlegraph/internal/config"
	"github.com/settlegraph/settlegraph/internal/service"
)

func main() {
	if err := service.Run(config.BankSim); err != nil {
		log.Fatal(err)
	}
}
