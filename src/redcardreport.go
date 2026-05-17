package main

import (
	"fmt"
	"os"

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
	//webhookURL := os.Getenv("webhookURL")
	databaseURL := os.Getenv("databaseURL")

	//yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	yesterday := "2026-01-01"
	baseURL := fmt.Sprintf("https://livescore-api.com/api-client/matches/history.json?key=%s&secret=%s&from=%s&to=%s", apiKey, apiSecret, yesterday, yesterday)

	totalPages, allMatches, err := core.FetchAllMatches(baseURL)
	if err != nil {
		fmt.Printf("Error fetching matches: %v\n", err)
		return
	}
	fmt.Printf("Fetched %d matches across %d pages\n", len(allMatches), totalPages)

	var redCardMatches []objects.RedCardMatch
	var teamRedCards []objects.TeamRedCards
	var playerRedCards []objects.PlayerRedCards
	totalRedCards := 0

	for i, match := range allMatches {
		homeTeam := objects.Team{Id: match.Home.Id, Name: match.Home.Name}
		awayTeam := objects.Team{Id: match.Away.Id, Name: match.Away.Name}

		score := match.Scores.FullTimeScore
		country := "Unknown"
		league := "Unknown"

		teamRedCards = append(teamRedCards, objects.TeamRedCards{Id: homeTeam.Id, Name: homeTeam.Name, RedCards: 0})
		teamRedCards = append(teamRedCards, objects.TeamRedCards{Id: awayTeam.Id, Name: awayTeam.Name, RedCards: 0})

		if match.Country != nil && match.Country.Name != "" {
			country = match.Country.Name
		}
		if match.Competition != nil && match.Competition.Name != "" {
			league = match.Competition.Name
		}

		fmt.Printf("Processing match %d/%d: %s vs %s\n", i+1, len(allMatches), homeTeam.Name, awayTeam.Name)

		eventsURL := fmt.Sprintf("%s&key=%s&secret=%s", match.Urls.Events, apiKey, apiSecret)
		events, err := core.FetchMatchEvents(eventsURL)
		if err != nil {
			fmt.Printf("Error fetching events for match %s vs %s: %v\n", homeTeam.Name, awayTeam.Name, err)
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
				matchEvents = append(matchEvents, fmt.Sprintf("⚽ %s' - %s (%s)", event.Time, event.Player, team.Name))
			case "OWN_GOAL":
				matchEvents = append(matchEvents, fmt.Sprintf("⚽ %s' - %s (%s) [OWN GOAL]", event.Time, event.Player, team.Name))
			case "GOAL_PENALTY":
				matchEvents = append(matchEvents, fmt.Sprintf("⚽ %s' - %s (%s) [GOAL_PENALTY]", event.Time, event.Player, team.Name))
			case "RED_CARD":
				matchEvents = append(matchEvents, fmt.Sprintf("🟥 %s' - %s (%s)", event.Time, event.Player, team.Name))
				matchRedCardCount++
				for j := range teamRedCards {
					if teamRedCards[j].Id == team.Id {
						teamRedCards[j].RedCards++
						playerFound := false
						for k := range playerRedCards {
							if playerRedCards[k].Name == event.Player && playerRedCards[k].TeamID == team.Id {
								playerRedCards[k].RedCards++
								playerFound = true
								break
							}
						}
						if !playerFound {
							playerRedCards = append(playerRedCards, objects.PlayerRedCards{Name: event.Player, TeamID: team.Id, RedCards: 1})
						}
						break
					}
				}
			case "YELLOW_RED_CARD":
				matchEvents = append(matchEvents, fmt.Sprintf("🟨🟥 %s' - %s (%s) [SECOND YELLOW CARD]", event.Time, event.Player, team.Name))
				matchRedCardCount++
				for j := range teamRedCards {
					if teamRedCards[j].Id == team.Id {
						teamRedCards[j].RedCards++
						playerFound := false
						for k := range playerRedCards {
							if playerRedCards[k].Name == event.Player && playerRedCards[k].TeamID == team.Id {
								playerRedCards[k].RedCards++
								playerFound = true
								break
							}
						}
						if !playerFound {
							playerRedCards = append(playerRedCards, objects.PlayerRedCards{Name: event.Player, TeamID: team.Id, RedCards: 1})
						}
						break
					}
				}
			}
		}

		if matchRedCardCount > 0 {
			totalRedCards += matchRedCardCount
			redCardMatches = append(redCardMatches, objects.RedCardMatch{
				Home:     homeTeam.Name,
				Away:     awayTeam.Name,
				Score:    score,
				Country:  country,
				League:   league,
				Events:   matchEvents,
				RedCards: matchRedCardCount,
			})
		}
	}

	// reportText := core.FormatReportText(yesterday, totalRedCards, redCardMatches)

	// filename := yesterday + "_matches.txt"
	// message := fmt.Sprintf("Red card report for %s: %d total red cards in %d matches", yesterday, totalRedCards, len(redCardMatches))
	// if err := core.SendDiscordWebhookWithFile(webhookURL, message, filename, []byte(reportText)); err != nil {
	// 	fmt.Println("Error sending webhook:", err)
	// 	return
	// }

	// fmt.Println("Webhook sent successfully!")

	fmt.Println("Updating red card database...")
	if err := core.UpdateRedCardDatabase(teamRedCards, playerRedCards, databaseURL); err != nil {
		fmt.Println("Error updating database:", err)
		return
	}
	fmt.Println("Database updated successfully!")
}
