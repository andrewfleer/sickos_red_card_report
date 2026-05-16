package core

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strings"

	"sickos_red_card_report/objects"
)

// FetchAllMatches retrieves all matches for a given date range
func FetchAllMatches(baseURL string) (int, []objects.Match, error) {
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

// FetchMatchEvents retrieves events for a specific match
func FetchMatchEvents(url string) ([]objects.Event, error) {
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

// FormatReportText generates a text report of red card matches
func FormatReportText(date string, totalRedCards int, matches []objects.RedCardMatch) string {
	var buf strings.Builder

	buf.WriteString(strings.Repeat("=", 60) + "\n")
	buf.WriteString(strings.ToUpper(date) + "'S MATCHES WITH RED CARDS\n")
	buf.WriteString(strings.Repeat("=", 60) + "\n")
	buf.WriteString(fmt.Sprintf("Total red cards: %d\n", totalRedCards))
	buf.WriteString(fmt.Sprintf("Matches with red cards: %d\n", len(matches)))
	buf.WriteString(strings.Repeat("=", 60) + "\n\n")

	for _, m := range matches {
		buf.WriteString(fmt.Sprintf("%s vs %s | Score: %s | Country: %s | League: %s | Red cards: %d\n", m.Home, m.Away, m.Score, m.Country, m.League, m.RedCards))
		for _, line := range m.Events {
			buf.WriteString(line + "\n")
		}
		buf.WriteString("\n")
	}

	return buf.String()
}

// SendDiscordWebhook sends a formatted message to a Discord webhook
func SendDiscordWebhook(webhookURL, content string) error {
	payload := map[string]string{"content": content}
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	resp, err := http.Post(webhookURL, "application/json", bytes.NewBuffer(payloadJSON))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("webhook request failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

// SendDiscordWebhookWithFile sends a message with a file attachment to Discord
func SendDiscordWebhookWithFile(webhookURL, content, filename string, fileContent []byte) error {
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

	part, err := writer.CreateFormFile("file", filepath.Base(filename))
	if err != nil {
		return err
	}
	if _, err := io.Copy(part, bytes.NewBuffer(fileContent)); err != nil {
		return err
	}

	if err := writer.Close(); err != nil {
		return err
	}

	resp, err := http.Post(webhookURL, writer.FormDataContentType(), body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("webhook request failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}
