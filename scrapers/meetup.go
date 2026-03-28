package scrapers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"
)

const gqlUrl = "https://www.meetup.com/gql2"

type Meetup struct {
	log *slog.Logger
	cli *http.Client
}

// TopicCategory are the categories that Meetup sorts events into.
type TopicCategory string

const (
	NewGroups            TopicCategory = "-999"
	SocialActivities     TopicCategory = "652"
	Hobbies              TopicCategory = "571"
	Sports               TopicCategory = "482"
	TravelOutdoors       TopicCategory = "684"
	Career               TopicCategory = "405"
	Technology           TopicCategory = "546"
	Community            TopicCategory = "604"
	IdentityLanguage     TopicCategory = "622"
	Games                TopicCategory = "535"
	Dance                TopicCategory = "612"
	Coaching             TopicCategory = "449"
	Music                TopicCategory = "395"
	Health               TopicCategory = "511"
	Art                  TopicCategory = "521"
	Science              TopicCategory = "436"
	Pets                 TopicCategory = "701"
	ReligionSpirituality TopicCategory = "593"
	Writing              TopicCategory = "467"
	Parents              TopicCategory = "673"
	Politics             TopicCategory = "642"
)

type RecommendedEventsVariables struct {
	First             int           `json:"first"`
	Lat               float64       `json:"lat"`
	Lon               float64       `json:"lon"`
	DataConfiguration string        `json:"dataConfiguration"`
	TopicCategory     TopicCategory `json:"topicCategoryId,omitzero"`
	// This should always be "RELEVANCE"
	SortField string `json:"sortField"`
}

type PersistedQuery struct {
	Version    int    `json:"version"`
	Sha256Hash string `json:"sha256Hash"`
}

type MeetupExtensions struct {
	PersistedQuery PersistedQuery `json:"persistedQuery"`
}

type OperationName string

const (
	LocationSearch              OperationName = "getLocationSearch"
	RecommendedEventsWithSeries OperationName = "recommendedEventsWithSeries"
)

type MeetupOperation struct {
	OperationName OperationName    `json:"operationName"`
	Variables     any              `json:"variables"`
	Extensions    MeetupExtensions `json:"extensions"`
}

type LocationSearchResult struct {
	Lat float64 `json:"lat"`
	Lon float64 `json:"lon"`
}

type LocationSearchResponse struct {
	Data struct {
		Result []LocationSearchResult `json:"result"`
	} `json:"data"`
}

type Event struct {
	Title       string    `json:"title"`
	DateTime    time.Time `json:"dateTime"`
	Description string    `json:"description"`
	EventType   EventType `json:"eventType"`
	EventUrl    string    `json:"eventUrl"`
}

type RecommendedEventsResponse struct {
	Data struct {
		Result Result `json:"result"`
	}
}

// ScrapeVars are options that can be passed into the meetup scrap function call.
type ScrapeVars struct {
	// Lat is the latitude to search around
	Lat float64
	// Lon is the longitude to search around
	Lon float64
	// Size is the number of events to return
	Size int
	// Cursor is the cursor for a specific page of result
	Cursor string
}

type DataPayload struct {
	Result Result `json:"result"`
}

type Result struct {
	TotalCount int      `json:"totalCount"`
	Events     []Edge   `json:"edges"`
	PageInfo   PageInfo `json:"pageInfo"`
}

type PageInfo struct {
	HasNextPage bool   `json:"hasNextPage"`
	EndCursor   string `json:"endCursor"`
}

type Edge struct {
	Node Event `json:"node"`
}

// EventType either describes an ONLINE or PHYSICAL event
type EventType string

const (
	Online   EventType = "ONLINE"
	Physical EventType = "PHYSICAL"
)

func newOperation(name OperationName, vars any) MeetupOperation {
	return MeetupOperation{
		OperationName: name,
		Variables:     vars,
		Extensions: MeetupExtensions{
			PersistedQuery: PersistedQuery{
				Version:    1,
				Sha256Hash: OperationShaMap[name],
			},
		},
	}
}

var OperationShaMap = map[OperationName]string{
	RecommendedEventsWithSeries: "3f7480361301be1b3208df0cd724930a22f7741d3c24666ab5b37a381ff4e0e8",
	LocationSearch:              "9c04de696e6a6697b82523524e59c52448f569689ed93b0821c9ff6437dc2089",
}

func (e *EventType) MarshalJSON() ([]byte, error) {
	return json.Marshal(string(*e))
}

func (e *EventType) UnmarshalJSON(bytes []byte) error {
	var resString string
	err := json.Unmarshal(bytes, &resString)
	if err != nil {
		return err
	}
	switch EventType(resString) {
	case Online, Physical:
		*e = EventType(resString)
		return nil
	}
	return fmt.Errorf("got unknown event type: %s", resString)
}

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

func (m *Meetup) GetLatAndLon(ctx context.Context) (float64, float64, error) {
	op := newOperation(LocationSearch, struct{}{})
	body, err := json.Marshal(op)
	if err != nil {
		return 0, 0, fmt.Errorf("error marshalling location search payload: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, gqlUrl, bytes.NewReader(body))
	if err != nil {
		return 0, 0, fmt.Errorf("error forming location search request: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Content-Type", "application/json")
	res, err := m.cli.Do(req)
	if err != nil {
		return 0, 0, fmt.Errorf("error performing location search request: %w", err)
	}
	defer res.Body.Close()
	var locationRes LocationSearchResponse
	if err = json.NewDecoder(res.Body).Decode(&locationRes); err != nil {
		return 0, 0, fmt.Errorf("error decoding location search response: %w", err)
	}
	if len(locationRes.Data.Result) == 0 {
		return 0, 0, fmt.Errorf("no location results returned")
	}
	result := locationRes.Data.Result[0]
	m.log.Debug("found location", slog.Float64("lat", result.Lat), slog.Float64("lon", result.Lon))
	return result.Lat, result.Lon, nil
}

func (m *Meetup) Scrape(ctx context.Context, scrapeVars ScrapeVars) error {
	op := newOperation(RecommendedEventsWithSeries, RecommendedEventsVariables{
		First:             scrapeVars.Size,
		Lat:               scrapeVars.Lat,
		Lon:               scrapeVars.Lon,
		DataConfiguration: `{"isSimplifiedSearchEnabled":true}`,
		TopicCategory:     Games,
		SortField:         "RELEVANCE",
	})
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
	defer res.Body.Close()
	m.log.Debug("Got response from gql", slog.Int("statusCode", res.StatusCode))
	dec := json.NewDecoder(res.Body)
	var rer RecommendedEventsResponse
	err = dec.Decode(&rer)
	if err != nil {
		return fmt.Errorf("error decoding response: %w", err)
	}
	// TODO: remove this debug file
	file, err := os.Create("test_decoded.json")
	if err != nil {
		return fmt.Errorf("error with creating test_decoded.json: %w", err)
	}
	defer file.Close()
	enc := json.NewEncoder(file)
	enc.SetIndent("", "    ")
	err = enc.Encode(rer)
	if err != nil {
		return fmt.Errorf("error with encoding to file: %w", err)
	}
	m.log.Debug("Scrape completed successfully", slog.Int("event_size", len(rer.Data.Result.Events)))
	return nil
}
