package scrapers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"time"
)

const (
	homeUrl = "https://www.meetup.com/find"
	gqlUrl  = "https://www.meetup.com/gql2"
)

type Meetup struct {
	log *slog.Logger
	cli *http.Client
}

type MeetupQueryVars struct {
	First int     `json:"first"`
	Lat   float64 `json:"lat"`
	Lon   float64 `json:"lon"`
}

type PersistedQuery struct {
	Version    int    `json:"version"`
	Sha256Hash string `json:"sha256Hash"`
}

type MeetupExtensions struct {
	PersistedQuery PersistedQuery `json:"persistedQuery"`
}

type MeetupOperation struct {
	OperationName string           `json:"operationName"`
	Variables     MeetupQueryVars  `json:"variables"`
	Extensions    MeetupExtensions `json:"extensions"`
}

const recommendedEventsHash = "3f7480361301be1b3208df0cd724930a22f7741d3c24666ab5b37a381ff4e0e8"

func NewMeetup() *Meetup {
	cli := &http.Client{
		Timeout: 15 * time.Second,
	}
	s := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	s = s.With("scraper_type", "meetup")
	return &Meetup{
		log: s,
		cli: cli,
	}
}

func (m *Meetup) getLatAndLon(ctx context.Context) (lat float64, lon float64, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, homeUrl, nil)
	if err != nil {
		return 0, 0, fmt.Errorf("error forming request: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	res, err := m.cli.Do(req)
	if err != nil {
		return 0, 0, fmt.Errorf("error reading response: %w", err)
	}
	defer res.Body.Close()
	bytes, err := io.ReadAll(res.Body)
	if err != nil {
		return 0, 0, fmt.Errorf("error getting home bytes: %w", err)
	}
	err = os.WriteFile("meetup_captured.html", bytes, 0666)
	if err != nil {
		return 0, 0, fmt.Errorf("error writing home file: %w", err)
	}
	latRe := regexp.MustCompile(`"lat":\s*([-\d.]+)`)
	lonRe := regexp.MustCompile(`"lon":\s*([-\d.]+)`)
	if match := latRe.FindSubmatch(bytes); match != nil {
		lat, err = strconv.ParseFloat(string(match[1]), 64)
		if err != nil {
			return 0, 0, fmt.Errorf("error parsing lat: %w", err)
		}
		m.log.Debug("found lat", slog.Float64("lat", lat))
	} else {
		return 0, 0, errors.New("latitude not found from meetup")
	}
	if match := lonRe.FindSubmatch(bytes); match != nil {
		lon, err = strconv.ParseFloat(string(match[1]), 64)
		if err != nil {
			return 0, 0, fmt.Errorf("error parsing lon: %w", err)
		}
		m.log.Debug("found lon", slog.Float64("lon", lat))
	} else {
		return 0, 0, errors.New("latitude not found from meetup")
	}
	return lat, lon, nil
}

func (m *Meetup) Scrape(ctx context.Context) error {
	lat, lon, err := m.getLatAndLon(ctx)
	if err != nil {
		return fmt.Errorf("error trying to fetch lat and lon from meetup: %w", err)
	}
	op := MeetupOperation{
		OperationName: "recommendedEventsWithSeries",
		Variables: MeetupQueryVars{
			First: 12,
			Lat:   lat,
			Lon:   lon,
		},
		Extensions: MeetupExtensions{
			PersistedQuery: PersistedQuery{
				Version:    1,
				Sha256Hash: recommendedEventsHash,
			},
		},
	}
	body, err := json.Marshal(op)
	if err != nil {
		return fmt.Errorf("error marshalling gql payload: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, gqlUrl, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("error forming request: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Content-Type", "application/json")
	res, err := m.cli.Do(req)
	if err != nil {
		return fmt.Errorf("error reading response: %w", err)
	}
	m.log.Debug("Got response from gql", slog.Int("statusCode", res.StatusCode))
	bytes, err := io.ReadAll(res.Body)
	if err != nil {
		return fmt.Errorf("error getting gql bytes: %w", err)
	}
	err = os.WriteFile("meetup_gql.json", bytes, 0666)
	defer res.Body.Close()
	return nil
}
