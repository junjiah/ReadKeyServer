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

// Must be called before other functions.
func Setup(store libstore.RedisStrore) {
	rs = store
}

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
	Link    string `json:"link" redis:"link"`
	Content string `json:"content" redis:"content"`
}

func GetFeedSourceSubscribers(srcId string) []string {
	c := rs.GetConnection()
	defer c.Close()

	subKey := util.FormatSubscriberKey(srcId)
	subers, err := redis.Strings(c.Do("LRANGE", subKey, 0, -1))
	if err != nil {
		// TODO: Detailed log & retry.
		log.Printf("[e] Failed to get feed source subscribers: \n", err.Error())
		return nil
	}

	return subers
}

func AddFeedSourceSubscriber(srcId, user string) {
	c := rs.GetConnection()
	defer c.Close()

	subKey := util.FormatSubscriberKey(srcId)
	if _, err := c.Do("RPUSH", subKey, user); err != nil {
		// TODO: Detailed log & retry.
		log.Printf("[e] Failed to append subscriber to a feed source.\n")
	}
}

func GetFeedEntriesFromSource(srcId string, feedIds []string) []FeedItemEntry {
	c := rs.GetConnection()
	defer c.Close()

	entries, err := redis.Strings(c.Do("HMGET", redis.Args{}.Add(srcId).AddFlat(feedIds)...))
	if err != nil {
		// TODO: Detailed log & retry.
		log.Printf("[e] Failed to get feed entries from a source.\n")
		return nil
	}

	var fe FeedItemEntry
	res := make([]FeedItemEntry, 0, len(entries))
	for _, entry := range entries {
		json.Unmarshal([]byte(entry), &fe)
		res = append(res, fe)
	}
	return res
}

func AppendLatestFeedIdToSource(srcId, feedId string) {
	c := rs.GetConnection()
	defer c.Close()

	latestKey := util.FormatLatestFeedsKey(srcId)
	c.Send("MULTI")
	c.Send("LPUSH", latestKey, feedId)
	c.Send("LTRIM", latestKey, 0, latestFeedCapacity-1)
	if _, err := c.Do("EXEC"); err != nil {
		// TODO: Detailed log & retry.
		log.Printf("[e] Failed to push latest feed ID to a source.\n")
	}
}

func GetLatestFeedIdsFromSource(srcId string) []string {
	c := rs.GetConnection()
	defer c.Close()

	latestKey := util.FormatLatestFeedsKey(srcId)
	feedIds, err := redis.Strings(c.Do("LRANGE", latestKey, 0, initUnreadFeedCount-1))
	if err != nil {
		// TODO: Detailed log & retry.
		log.Printf("[e] Failed to get latest feed IDs from a source.\n")
		return nil
	}
	return feedIds
}

func AddFeedEntryToSource(srcId string, fe FeedItemEntry) {
	c := rs.GetConnection()
	defer c.Close()

	fePacket, _ := json.Marshal(fe)
	if _, err := c.Do("HSET", srcId, fe.FeedId, fePacket); err != nil {
		// TODO: Detailed log & retry.
		log.Printf("[e] Failed to add feed entry to a source.\n")
	}
}

func GetFeed(feedId string) FeedItem {
	c := rs.GetConnection()
	defer c.Close()

	var fi FeedItem
	v, err := redis.Values(c.Do("HGETALL", feedId))
	if err != nil {
		// TODO: Detailed log & retry.
		log.Printf("[e] Failed to get a feed item.\n")
		return FeedItem{}
	}

	if err := redis.ScanStruct(v, &fi); err != nil {
		// TODO: Detailed log & retry.
		log.Printf("[e] Failed to scan the feed item.\n")
		return FeedItem{}
	}

	return fi
}

func SetFeed(feedId string, fi FeedItem) {
	c := rs.GetConnection()
	defer c.Close()

	if _, err := c.Do("HMSET", redis.Args{}.Add(feedId).AddFlat(&fi)...); err != nil {
		// TODO: Detailed log & retry.
		log.Printf("[e] Failed to set a feed item.\n")
	}
}
