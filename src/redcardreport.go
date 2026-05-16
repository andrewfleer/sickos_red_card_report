package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"sickos_red_card_report/objects"

	"github.com/joho/godotenv"
)

func main() {
	//load secrets from .env file
	err := godotenv.Load("secrets.env")
	if err != nil {
		fmt.Println("Error loading .env file:", err)
		return
	}

	apiKey := os.Getenv("apiKey")
	apiSecret := os.Getenv("apiSecret")
	webhookURL := os.Getenv("webhookURL")

	yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	baseURL := fmt.Sprintf("https://livescore-api.com/api-client/matches/history.json?key=%s&secret=%s&from=%s&to=%s", apiKey, apiSecret, yesterday, yesterday)

	totalPages, allMatches, err := fetchAllMatches(baseURL)
	if err != nil {
		fmt.Printf("Error fetching matches: %v\n", err)
		return
	}
	fmt.Printf("Fetched %d matches across %d pages\n", len(allMatches), totalPages)

	var redCardMatches []objects.RedCardMatch
	totalRedCards := 0

	for i, match := range allMatches {
		homeTeam := match.Home.Name
		awayTeam := match.Away.Name
		score := match.Scores.FullTimeScore
		country := "Unknown"
		league := "Unknown"

		if match.Country != nil && match.Country.Name != "" {
			country = match.Country.Name
		}
		if match.Competition != nil && match.Competition.Name != "" {
			league = match.Competition.Name
		}

		fmt.Printf("Processing match %d/%d: %s vs %s\n", i+1, len(allMatches), homeTeam, awayTeam)

		eventsURL := fmt.Sprintf("%s&key=%s&secret=%s", match.Urls.Events, apiKey, apiSecret)
		events, err := fetchMatchEvents(eventsURL)
		if err != nil {
			fmt.Printf("Error fetching events for match %s vs %s: %v\n", homeTeam, awayTeam, err)
			continue
		}

		var matchEvents []string
		matchRedCardCount := 0

		for _, event := range events {
			team := awayTeam
			if event.HomeAway == "h" {
				team = homeTeam
			}

			switch event.Type {
			case "GOAL":
				matchEvents = append(matchEvents, fmt.Sprintf("⚽ %s' - %s (%s)", event.Time, event.Player, team))
			case "OWN_GOAL":
				matchEvents = append(matchEvents, fmt.Sprintf("⚽ %s' - %s (%s) [OWN GOAL]", event.Time, event.Player, team))
			case "GOAL_PENALTY":
				matchEvents = append(matchEvents, fmt.Sprintf("⚽ %s' - %s (%s) [GOAL_PENALTY]", event.Time, event.Player, team))
			case "RED_CARD":
				matchEvents = append(matchEvents, fmt.Sprintf("🟥 %s' - %s (%s)", event.Time, event.Player, team))
				matchRedCardCount++
			case "YELLOW_RED_CARD":
				matchEvents = append(matchEvents, fmt.Sprintf("🟨🟥 %s' - %s (%s) [SECOND YELLOW CARD]", event.Time, event.Player, team))
				matchRedCardCount++
			}
		}

		if matchRedCardCount > 0 {
			totalRedCards += matchRedCardCount
			redCardMatches = append(redCardMatches, objects.RedCardMatch{
				Home:     homeTeam,
				Away:     awayTeam,
				Score:    score,
				Country:  country,
				League:   league,
				Events:   matchEvents,
				RedCards: matchRedCardCount,
			})
		}
	}

	filename := fmt.Sprintf("%s_matches.txt", yesterday)
	if err := writeReport(filename, yesterday, totalRedCards, redCardMatches); err != nil {
		fmt.Printf("Error writing report: %v\n", err)
		return
	}

	fmt.Printf("Report written to %s\n", filename)

	content := fmt.Sprintf("Total red cards on %s: %d\nMatches with red cards: %d", yesterday, totalRedCards, len(redCardMatches))
	if err := sendDiscordWebhookWithFile(webhookURL, content, filename); err != nil {
		fmt.Println("Error sending webhook:", err)
		return
	}

	fmt.Println("Webhook sent successfully!")

}

func fetchAllMatches(baseURL string) (int, []objects.Match, error) {
	page1URL := baseURL + "&page=1"
	resp, err := http.Get(page1URL)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, nil, fmt.Errorf("API request failed with status: %s", resp.Status)
	}

	var apiResp objects.ApiResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return 0, nil, err
	}

	totalPages := 1
	if apiResp.Data != nil && apiResp.Data.TotalPages > 0 {
		totalPages = apiResp.Data.TotalPages
	}

	matches := make([]objects.Match, 0)
	if apiResp.Data != nil {
		matches = append(matches, apiResp.Data.Match...)
	}

	for page := 2; page <= totalPages; page++ {
		url := fmt.Sprintf("%s&page=%d", baseURL, page)
		resp, err := http.Get(url)
		if err != nil {
			fmt.Printf("Error fetching page %d: %v\n", page, err)
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			fmt.Printf("Error fetching page %d: %s\n", page, resp.Status)
			continue
		}

		var pageResp objects.ApiResponse
		if err := json.NewDecoder(resp.Body).Decode(&pageResp); err != nil {
			fmt.Printf("Error decoding page %d: %v\n", page, err)
			continue
		}

		if pageResp.Data != nil {
			matches = append(matches, pageResp.Data.Match...)
		}
	}
	return totalPages, matches, nil
}

func fetchMatchEvents(url string) ([]objects.Event, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request failed with status: %s", resp.Status)
	}

	var eventsResp objects.EventsResponse
	if err := json.NewDecoder(resp.Body).Decode(&eventsResp); err != nil {
		return nil, err
	}

	if eventsResp.Data == nil {
		return nil, nil
	}
	return eventsResp.Data.Events, nil
}

func writeReport(filename, date string, totalRedCards int, matches []objects.RedCardMatch) error {
	file, err := os.Create("../data/" + filename)
	if err != nil {
		return err
	}
	defer file.Close()

	fmt.Fprintln(file, strings.Repeat("=", 60))
	fmt.Fprintf(file, "%s'S MATCHES WITH RED CARDS\n", date)
	fmt.Fprintln(file, strings.Repeat("=", 60))
	fmt.Fprintf(file, "Total red cards: %d\n", totalRedCards)
	fmt.Fprintf(file, "Matches with red cards: %d\n", len(matches))
	fmt.Fprintln(file, strings.Repeat("=", 60))
	fmt.Fprintln(file)

	for _, m := range matches {
		fmt.Fprintf(file, "%s vs %s | Score: %s | Country: %s | League: %s | Red cards: %d\n", m.Home, m.Away, m.Score, m.Country, m.League, m.RedCards)
		for _, line := range m.Events {
			fmt.Fprintln(file, line)
		}
		fmt.Fprintln(file)
	}
	return nil
}

func sendDiscordWebhookWithFile(webhookURL, content, filename string) error {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	payload := map[string]string{"content": content}
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	if err := writer.WriteField("payload_json", string(payloadJSON)); err != nil {
		return err
	}

	file, err := os.Open("../data/" + filename)
	if err != nil {
		return err
	}
	defer file.Close()

	part, err := writer.CreateFormFile("file", filepath.Base(filename))
	if err != nil {
		return err
	}
	if _, err := io.Copy(part, file); err != nil {
		return err
	}

	if err := writer.Close(); err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, webhookURL, body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("discord webhook returned %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
}
