package objects

type Match struct {
	Home        Team       `json:"home"`
	Away        Team       `json:"away"`
	Scores      Score      `json:"scores"`
	Country     *NamedItem `json:"country"`
	Competition *NamedItem `json:"competition"`
	Urls        EventURLs  `json:"urls"`
}
