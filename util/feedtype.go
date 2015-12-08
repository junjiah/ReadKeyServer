package util

type FeedSource struct {
	SourceId string `json:"id"`
	Title    string `json:"title"`
}

type FeedItemEntry struct {
	FeedId   string `json:"id"`
	Title    string `json:"title"`
	Keywords string `json:"keywords"`
	PubDate  string `json:"pubDate"`
}

type FeedItem struct {
	Link    string `json:"link"`
	Content string `json:"content"`
}
