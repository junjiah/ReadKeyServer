package feed

import (
	"encoding/json"
	"log"

	"github.com/edfward/readkey/libstore"
	"github.com/edfward/readkey/util"

	"github.com/garyburd/redigo/redis"
)

const (
	latestFeedCapacity  = 100
	initUnreadFeedCount = 15
)

var rs libstore.RedisStrore

// Setup must be called before other functions to configure the Redis store.
func Setup(store libstore.RedisStrore) {
	rs = store
}

// Source describes a site's feed source (Atom or RSS).
// Serialized as JSON, not in Redis top level.
type Source struct {
	SourceID string `json:"id"`
	Title    string `json:"title"`
	URL      string `json:"url"`
}

// ItemEntry describes an entry struct to the actual feed item, so only contains
// a subset of the information. Serialized as JSON, not in Redis top level.
type ItemEntry struct {
	FeedID   string `json:"id"`
	Title    string `json:"title"`
	Keywords string `json:"keywords"`
	PubDate  string `json:"pubDate"`
}

// Item keeps the actual feed item, which are stored into top-level Redis
type Item struct {
	Link    string `json:"link" redis:"link"`
	Content string `json:"content" redis:"content"`
}

// GetSourceSubscribers retrieves subscribed user IDs.
// TODO: Currently even if a user unsubscribes a feed source, the subscriber list
// of that source doesn't remove that user.
func GetSourceSubscribers(srcID string) []string {
	c := rs.GetConnection()
	defer c.Close()

	subKey := util.FormatSubscriberKey(srcID)
	subers, err := redis.Strings(c.Do("LRANGE", subKey, 0, -1))
	if err != nil {
		// TODO: Detailed log & retry.
		log.Printf("[e] Failed to get feed source subscribers: %v\n", err.Error())
		return nil
	}

	return subers
}

// AddSourceSubscriber adds a user to a feed source's subscriber list.
func AddSourceSubscriber(srcID, user string) {
	c := rs.GetConnection()
	defer c.Close()

	subKey := util.FormatSubscriberKey(srcID)
	if _, err := c.Do("RPUSH", subKey, user); err != nil {
		// TODO: Detailed log & retry.
		log.Printf("[e] Failed to append subscriber to a feed source.\n")
	}
}

// GetItemEntriesFromSource returns a list of feed item entries given a list
// of feed IDs.
func GetItemEntriesFromSource(srcID string, feedIDs []string) []ItemEntry {
	c := rs.GetConnection()
	defer c.Close()

	if len(feedIDs) == 0 {
		return nil
	}

	entries, err := redis.Strings(c.Do("HMGET", redis.Args{}.Add(srcID).AddFlat(feedIDs)...))
	if err != nil {
		// TODO: Detailed log & retry.
		log.Printf("[e] Failed to get feed entries from a source.\n")
		return nil
	}

	var fe ItemEntry
	res := make([]ItemEntry, 0, len(entries))
	for _, entry := range entries {
		json.Unmarshal([]byte(entry), &fe)
		res = append(res, fe)
	}
	return res
}

// AppendLatestItemIDToSource appends a feed ID to the latest queue (a capped list) of a feed source.
func AppendLatestItemIDToSource(srcID, feedID string) {
	c := rs.GetConnection()
	defer c.Close()

	latestKey := util.FormatLatestFeedsKey(srcID)
	c.Send("MULTI")
	c.Send("LPUSH", latestKey, feedID)
	c.Send("LTRIM", latestKey, 0, latestFeedCapacity-1)
	if _, err := c.Do("EXEC"); err != nil {
		// TODO: Detailed log & retry.
		log.Printf("[e] Failed to push latest feed ID to a source.\n")
	}
}

// GetLatestItemIdsFromSource fetches the latest feed IDs of a feed source.
func GetLatestItemIdsFromSource(srcID string) []string {
	c := rs.GetConnection()
	defer c.Close()

	latestKey := util.FormatLatestFeedsKey(srcID)
	feedIDs, err := redis.Strings(c.Do("LRANGE", latestKey, 0, initUnreadFeedCount-1))
	if err != nil {
		// TODO: Detailed log & retry.
		log.Printf("[e] Failed to get latest feed IDs from a source.\n")
		return nil
	}
	return feedIDs
}

// AddItemEntryToSource adds a feed item entry to a feed source.
func AddItemEntryToSource(srcID string, fe ItemEntry) {
	c := rs.GetConnection()
	defer c.Close()

	fePacket, _ := json.Marshal(fe)
	if _, err := c.Do("HSET", srcID, fe.FeedID, fePacket); err != nil {
		// TODO: Detailed log & retry.
		log.Printf("[e] Failed to add feed entry to a source.\n")
	}
}

// GetItem retrieves the actual content of a feed.
func GetItem(feedID string) Item {
	c := rs.GetConnection()
	defer c.Close()

	var fi Item
	v, err := redis.Values(c.Do("HGETALL", feedID))
	if err != nil {
		// TODO: Detailed log & retry.
		log.Printf("[e] Failed to get a feed item.\n")
		return Item{}
	}

	if err := redis.ScanStruct(v, &fi); err != nil {
		// TODO: Detailed log & retry.
		log.Printf("[e] Failed to scan the feed item.\n")
		return Item{}
	}

	return fi
}

// SetItem sets the actual content of a feed.
func SetItem(feedID string, fi Item) {
	c := rs.GetConnection()
	defer c.Close()

	if _, err := c.Do("HMSET", redis.Args{}.Add(feedID).AddFlat(&fi)...); err != nil {
		// TODO: Detailed log & retry.
		log.Printf("[e] Failed to set a feed item.\n")
	}
}

// GetListeningSources fetches all listening feed sources.
// TODO: For now it only grows but never shrinks.
func GetListeningSources() []Source {
	c := rs.GetConnection()
	defer c.Close()

	srcs, err := redis.Strings(c.Do("SMEMBERS", util.FormatListeningKey()))
	if err != nil {
		// TODO: Detailed log & retry.
		log.Printf("[e] Failed to get all listening feed sources.\n")
		return nil
	}

	var fs Source
	res := make([]Source, 0, len(srcs))
	for _, src := range srcs {
		// Assume no unmarshalling error.
		json.Unmarshal([]byte(src), &fs)
		res = append(res, fs)
	}
	return res
}

// AppendListeningSource appends a feed source to the listening list.
func AppendListeningSource(src Source) {
	c := rs.GetConnection()
	defer c.Close()

	srcPacket, _ := json.Marshal(src)
	c.Do("SADD", util.FormatListeningKey(), srcPacket)
}
