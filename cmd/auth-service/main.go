package main

import (
	"context"
	"log"

	"simple_auth_by_doubao/internal/app"
)

func main() {
	if err := app.Run(context.Background()); err != nil {
		log.Fatal(err)
	}
}
