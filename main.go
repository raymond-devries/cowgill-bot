package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/jefflinse/githubsecret"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

type GithubPublicKey struct {
	KeyID string `json:"key_id"`
	Key   string `json:"key"`
}

type GithubSecret struct {
	EncryptedValue string `json:"encrypted_value"`
	KeyID          string `json:"key_id"`
}

type MapUrls struct {
	LightUrl string `json:"light_url"`
	DarkUrl  string `json:"dark_url"`
}

type Route struct {
	MapUrls MapUrls `json:"map_urls"`
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

func serializeJson[T any](value T) io.Reader {
	serializedValue, err := json.Marshal(value)
	if err != nil {
		log.Fatalln("Error serializing value")
	}
	return bytes.NewBuffer(serializedValue)
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
	client := &http.Client{}
	clientId := "137765"
	clientSecret := os.Getenv("STRAVA_CLIENT_SECRET")
	refreshToken := os.Getenv("STRAVA_REFRESH_TOKEN")
	formData := fmt.Sprintf("client_id=%s&client_secret=%s&grant_type=refresh_token&refresh_token=%s", clientId, clientSecret, refreshToken)
	data := strings.NewReader(formData)
	req, err := http.NewRequest("POST", "https://www.strava.com/api/v3/oauth/token", data)
	if err != nil {
		log.Fatalln("Error creating request")
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := client.Do(req)
	if err != nil {
		log.Fatalln("Error refreshing token")
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			log.Fatalln("Error closing body")
		}
	}(resp.Body)
	bodyText, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatalln("Error reading response body")
	}
	var auth Auth
	err = json.Unmarshal(bodyText, &auth)
	if err != nil {
		log.Fatalln("Error parsing auth data")
	}
	return auth
}

func getUpcomingStravaEvents(auth Auth) []Event {
	req, err := http.NewRequest("GET", "https://www.strava.com/api/v3/clubs/470714/group_events", nil)
	if err != nil {
		log.Fatalln("Error creating request")
	}
	req.Header.Set("Authorization", "Bearer "+auth.AccessToken)
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
	return upcomingEvents
}

func githubRequest[T any](url string, method string, body io.Reader) T {
	githubToken := os.Getenv("GH_TOKEN")

	req, err := http.NewRequest(method, "https://api.github.com/repos/raymond-devries/cowgill-bot"+url, body)
	if err != nil {
		log.Fatalf("Error creating github request for url %s", url)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+githubToken)
	req.Header.Set("X-Github-Api-Version", "2022-11-28")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Fatalf("Error making github request to %s", url)
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			log.Fatalln("Error closing body")
		}
	}(resp.Body)
	bodyText, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatalln("Error reading response body")
	}

	var response T
	if string(bodyText) == "" {
		return response
	}
	err = json.Unmarshal(bodyText, &response)
	if err != nil {
		log.Fatalln("Error parsing data")
	}
	return response
}

func updateStravaRefreshToken(refreshToken string) {
	if refreshToken == os.Getenv("STRAVA_REFRESH_TOKEN") {
		log.Println("Strava refresh token already up to date")
		return
	}
	publicKey := githubRequest[GithubPublicKey]("/actions/secrets/public-key", "GET", nil)
	encryptedRefreshToken, err := githubsecret.Encrypt(publicKey.Key, refreshToken)
	if err != nil {
		log.Fatalln("Error encrypting secret")
	}
	githubStravaRefreshToken := GithubSecret{
		EncryptedValue: encryptedRefreshToken,
		KeyID:          publicKey.KeyID,
	}
	githubRequest[struct{}]("/actions/secrets/STRAVA_REFRESH_TOKEN", "PUT", serializeJson[GithubSecret](githubStravaRefreshToken))
	log.Println("Successfully updated strava refresh token")
}

func main() {
	stravaAuth := getStravaAuth()
	updateStravaRefreshToken(stravaAuth.RefreshToken)
	upcomingEvents := getUpcomingStravaEvents(stravaAuth)
	// Print events for now
	fmt.Println(upcomingEvents)
}
