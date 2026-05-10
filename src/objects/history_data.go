package objects

type HistoryData struct {
	TotalPages int     `json:"total_pages"`
	Match      []Match `json:"match"`
}
