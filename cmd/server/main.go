package main

import (
	"log"

	_ "github.com/joho/godotenv/autoload"

	"wenbang/internal/app"
)

func main() {
	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
