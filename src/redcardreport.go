package main

import (
	"fmt"
	"os"
	"time"

	"sickos_red_card_report/core"
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

	totalPages, allMatches, err := core.FetchAllMatches(baseURL)
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
		events, err := core.FetchMatchEvents(eventsURL)
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

	reportText := core.FormatReportText(yesterday, totalRedCards, redCardMatches)
	fmt.Print(reportText)

	filename := yesterday + "_matches.txt"
	message := fmt.Sprintf("Red card report for %s: %d total red cards in %d matches", yesterday, totalRedCards, len(redCardMatches))
	if err := core.SendDiscordWebhookWithFile(webhookURL, message, filename, []byte(reportText)); err != nil {
		fmt.Println("Error sending webhook:", err)
		return
	}

	fmt.Println("Webhook sent successfully!")
}
