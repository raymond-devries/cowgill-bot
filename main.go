package main

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"time"
)

type MapUrls struct {
	LightUrl string `json:"light_url"`
	DarkUrl  string `json:"dark_url"`
}

type Route struct {
	MapUrls MapUrls `json:"map_urls"`
}

type Event struct {
	Address             string      `json:"address"`
	Description         string      `json:"description"`
	Id                  int         `json:"id"`
	Route               Route       `json:"route"`
	Title               string      `json:"title"`
	UpcomingOccurrences []time.Time `json:"upcoming_occurrences"`
	WomenOnly           bool        `json:"women_only"`
}

func timesAfterNow(times []time.Time) bool {
	now := time.Now()
	for _, t := range times {
		if t.After(now) {
			return true
		}
	}
	return false
}

func main() {
	req, err := http.NewRequest("GET", "https://www.strava.com/api/v3/clubs/470714/group_events", nil)
	if err != nil {
		log.Fatalln("Error creating request")
	}
	req.Header.Set("Authorization", "Bearer")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Fatalln("Error sending request")
	}
	if resp.StatusCode != 200 {
		log.Fatalf("Invalid status code: %d", resp.StatusCode)
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			log.Fatalln("Error closing body")
		}
	}(resp.Body)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatalln("Error reading response body")
	}

	var events []Event
	err = json.Unmarshal(body, &events)
	if err != nil {
		log.Fatalf("Error parsing JSON: %v", err)
	}

	var upcomingEvents []Event
	for _, event := range events {
		if timesAfterNow(event.UpcomingOccurrences) {
			upcomingEvents = append(upcomingEvents, event)
		}
	}
	log.Println("Successfully retrieved upcoming Cowgill events")
}
