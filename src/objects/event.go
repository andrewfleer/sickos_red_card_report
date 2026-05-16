package objects

type Event struct {
	Type     string `json:"event"`
	Player   string `json:"player"`
	Time     string `json:"time"`
	HomeAway string `json:"home_away"`
}

type Events struct {
	Events []Event `json:"event"`
}

type EventsResponse struct {
	Data *Events `json:"data"`
}

type EventURLs struct {
	Events string `json:"events"`
}
