package user

import (
	"encoding/json"
	"log"

	"github.com/edfward/readkey/libstore"
	"github.com/edfward/readkey/model/feed"
	"github.com/edfward/readkey/util"
	"github.com/garyburd/redigo/redis"
)

var rs libstore.RedisStrore

// Must be called before other functions.
func Setup(store libstore.RedisStrore) {
	rs = store
}

func GetFeedSubscriptions(user string) []feed.FeedSource {
	c := rs.GetConnection()
	defer c.Close()

	userSubKey := util.FormatUserSubsKey(user)
	srcs, err := redis.Strings(c.Do("HVALS", userSubKey))
	if err != nil {
		// TODO: Detailed log & retry.
		log.Printf("[e] Failed to get subscribed feed sources.\n")
		return nil
	}

	var fs feed.FeedSource
	res := make([]feed.FeedSource, 0, len(srcs))
	for _, src := range srcs {
		// Assume no unmarshalling error.
		json.Unmarshal([]byte(src), &fs)
		res = append(res, fs)
	}
	return res
}

func AppendFeedSubscription(user string, src feed.FeedSource) {
	c := rs.GetConnection()
	defer c.Close()

	userSubKey := util.FormatUserSubsKey(user)
	srcPacket, _ := json.Marshal(src)
	if _, err := c.Do("HSET", userSubKey, src.SourceId, srcPacket); err != nil {
		// TODO: Detailed log.
		log.Printf("[e] Failed to append feed subscription to user.\n")
	}
}

// Remove the subsribed feed source, return true if successful.
func RemoveFeedSubscription(user, srcId string) bool {
	c := rs.GetConnection()
	defer c.Close()

	userSubKey := util.FormatUserSubsKey(user)
	deleted, err := redis.Int64(c.Do("HDEL", userSubKey, srcId))
	if err != nil {
		// TODO: Detailed log & retry.
		log.Printf("[e] Failed to remove subscription.\n")
		return false
	}
	return deleted == 1
}

func GetUnreadFeedIds(user, srcId string) []string {
	c := rs.GetConnection()
	defer c.Close()

	unreadKey := util.FormatUserUnreadKey(user, srcId)
	unreadIds, err := redis.Strings(c.Do("SMEMBERS", unreadKey))
	if err != nil {
		// TODO: Detailed log & retry.
		log.Printf("[e] Failed to get unread feed IDs.\n")
		return nil
	}

	return unreadIds
}

func AppendUnreadFeedId(user, srcId, feedId string) {
	c := rs.GetConnection()
	defer c.Close()

	unreadKey := util.FormatUserUnreadKey(user, srcId)
	if _, err := c.Do("SADD", unreadKey, feedId); err != nil {
		// TODO: Detailed log & retry.
		log.Printf("[e] Failed to add an unread feed ID.\n")
	}
}

func RemoveUnreadFeedId(user, srcId, feedId string) {
	c := rs.GetConnection()
	defer c.Close()

	unreadKey := util.FormatUserUnreadKey(user, srcId)
	if _, err := c.Do("SREM", unreadKey, feedId); err != nil {
		// TODO: Detailed log & retry.
		log.Printf("[e] Failed to remove an unread feed ID.\n")
	}
}

// TODO: Maybe should be done when retrieving subscriptions?
func GetUnreadFeedCount(user, srcId string) int64 {
	c := rs.GetConnection()
	defer c.Close()

	unreadKey := util.FormatUserUnreadKey(user, srcId)
	cnt, err := redis.Int64(c.Do("SCARD", unreadKey))
	if err != nil {
		// TODO: Detailed log & retry.
		log.Printf("[e] Failed to get unread feed count.\n")
	}
	return cnt
}

// TODO: Will be deprecated by using *latest* with Redis's capped list.
func InitUserUnreadQueue(user, srcId string) {
	c := rs.GetConnection()
	defer c.Close()

	unreadKey := util.FormatUserUnreadKey(user, srcId)
	latestFeedIds := feed.GetLatestFeedIdsFromSource(srcId)

	if _, err := c.Do("SADD", redis.Args{}.Add(unreadKey).AddFlat(latestFeedIds)...); err != nil {
		// TODO: Detailed log & retry.
		log.Printf("[e] Failed to init unread queue for user.\n")
	}
}
