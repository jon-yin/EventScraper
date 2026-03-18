package main

import (
	"context"
	"log"

	"github.com/jon-yin/EventScraper/scrapers"
)

func main() {
	ctx := context.Background()
	muScraper := scrapers.NewMeetup()
	if err := muScraper.Scrape(ctx); err != nil {
		log.Fatal(err)
	}
}
