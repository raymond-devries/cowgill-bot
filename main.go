package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"log"
	"os"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/jefflinse/githubsecret"
)

type GithubPublicKey struct {
	KeyID string `json:"key_id"`
	Key   string `json:"key"`
}

type MapUrls struct {
	LightUrl string `json:"light_url"`
	DarkUrl  string `json:"dark_url"`
}

type Map struct {
	Id              string `json:"id"`
	SummaryPolyline string `json:"summary_polyline"`
}

type Route struct {
	MapUrls MapUrls `json:"map_urls"`
	Map     Map     `json:"map"`
}

type Auth struct {
	TokenType    string `json:"token_type"`
	AccessToken  string `json:"access_token"`
	ExpiresAt    int    `json:"expires_at"`
	RefreshToken string `json:"refresh_token"`
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

func getStravaAuth() Auth {
	client := resty.New()

	auth := Auth{}
	_, err := client.R().
		SetBody(map[string]string{
			"client_id":     "137765",
			"client_secret": os.Getenv("STRAVA_CLIENT_SECRET"),
			"grant_type":    "refresh_token",
			"refresh_token": os.Getenv("STRAVA_REFRESH_TOKEN")}).
		SetResult(&auth).
		Post("https://www.strava.com/api/v3/oauth/token")
	if err != nil {
		log.Fatalln("Failed to retrieve Strava credentials")
	}

	return auth
}

func getUpcomingStravaEvents(auth Auth) []Event {
	var events []Event

	client := resty.New()

	_, err := client.R().
		SetResult(&events).
		SetAuthToken(auth.AccessToken).
		Get("https://www.strava.com/api/v3/clubs/470714/group_events")

	if err != nil {
		log.Fatalln("Failed to retrieve Strava events")
	}

	var upcomingEvents []Event
	for _, event := range events {
		if timesAfterNow(event.UpcomingOccurrences) {
			upcomingEvents = append(upcomingEvents, event)
		}
	}
	return upcomingEvents
}

func updateStravaRefreshToken(refreshToken string) {
	if refreshToken == os.Getenv("STRAVA_REFRESH_TOKEN") {
		log.Println("Strava refresh token already up to date")
		return
	}

	client := resty.New()
	client.SetBaseURL("https://api.github.com/repos/raymond-devries/cowgill-bot")
	client.SetAuthToken(os.Getenv("GH_TOKEN"))
	client.SetHeader("Accept", "application/vnd.github+json")
	client.SetHeader("X-Github-Api-Version", "2022-11-28")

	publicKey := GithubPublicKey{}
	resp, err := client.R().
		SetResult(&publicKey).
		Get("/actions/secrets/public-key")
	if (err != nil) || (resp.StatusCode() != 200) {
		log.Fatalln("Failed to retrieve public key")
	}

	encryptedRefreshToken, err := githubsecret.Encrypt(publicKey.Key, refreshToken)
	if err != nil {
		log.Fatalln("Error encrypting secret")
	}

	resp, err = client.R().
		SetBody(map[string]string{"encrypted_value": encryptedRefreshToken, "key_id": publicKey.KeyID}).
		Put("/actions/secrets/STRAVA_REFRESH_TOKEN")
	if (err != nil) || (resp.StatusCode() != 204) {
		log.Fatalln("Failed to update github secret")
	}

	log.Println("Successfully updated strava refresh token")
}

func updateWebsiteUpcomingEvents(upcomingEvents []Event) {
	client := resty.New()
	client.SetBaseURL("https://api.github.com/repos/raymond-devries/cowgill")
	client.SetAuthToken(os.Getenv("GH_TOKEN"))
	client.SetHeader("Accept", "application/vnd.github+json")
	client.SetHeader("X-Github-Api-Version", "2022-11-28")

	currentFile := struct {
		EncodedContent string `json:"content"`
		Sha            string `json:"sha"`
	}{}
	resp, err := client.R().
		SetResult(&currentFile).
		Get("/contents/src/routes/events/upcoming_events.json")
	if (err != nil) || (resp.StatusCode() != 200) {
		log.Fatalln("Failed to retrieve upcoming_events.json from the website")
	}
	repoUpcomingEventsDecoded, err := base64.StdEncoding.DecodeString(currentFile.EncodedContent)
	if err != nil {
		log.Fatalln("Failed to decode upcoming_events.json from the website repo")
	}

	var repoUpcomingEvents []Event
	err = json.Unmarshal(repoUpcomingEventsDecoded, &repoUpcomingEvents)
	if err != nil {
		log.Fatalln("Failed to marshall upcoming_events.json from the website repo")
	}

	ignoreFields := cmpopts.IgnoreFields(Event{}, "Route.MapUrls")
	if cmp.Equal(upcomingEvents, repoUpcomingEvents, ignoreFields) {
		log.Println("Website events are already up to date")
		return
	}

	log.Println("Differences in upcoming events")
	fmt.Println(cmp.Diff(upcomingEvents, repoUpcomingEvents))

	newContents, err := json.MarshalIndent(upcomingEvents, "", "  ")
	if err != nil {
		log.Fatalln("Failed to serialize upcoming_events.json")
	}
	encodedNewContents := base64.StdEncoding.EncodeToString(newContents)

	resp, err = client.R().
		SetBody(map[string]string{
			"content": encodedNewContents, "message": "Update upcoming events", "sha": currentFile.Sha,
		}).
		Put("/contents/src/routes/events/upcoming_events.json")
	if (err != nil) || (resp.StatusCode() != 200) {
		log.Fatalln("Failed to update upcoming_events.json on the website")
	}

	log.Println("Successfully updated website upcoming events")

}

func main() {
	stravaAuth := getStravaAuth()
	updateStravaRefreshToken(stravaAuth.RefreshToken)
	upcomingEvents := getUpcomingStravaEvents(stravaAuth)
	fmt.Println(upcomingEvents)
}
