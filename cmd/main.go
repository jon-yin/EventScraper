package main

import (
	"context"
	"log"

	"github.com/jon-yin/EventScraper/scrapers"
)

func main() {
	ctx := context.Background()
	muScraper := scrapers.NewMeetup()
	lat, lon, err := muScraper.GetLatAndLon(ctx)
	if err != nil {
		log.Fatal(err)
	}
	if err := muScraper.Scrape(ctx, scrapers.ScrapeVars{
		Lat:  lat,
		Lon:  lon,
		Size: 50,
	}); err != nil {
		log.Fatal(err)
	}
}
