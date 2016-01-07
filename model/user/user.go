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

// Setup must be called before other functions to configure the Redis store.
func Setup(store libstore.RedisStrore) {
	rs = store
}

// GetFeedSubscriptions fetches all subscribed feed sources of a user.
func GetFeedSubscriptions(user string) []feed.Source {
	c := rs.GetConnection()
	defer c.Close()

	userSubKey := util.FormatUserSubsKey(user)
	srcs, err := redis.Strings(c.Do("HVALS", userSubKey))
	if err != nil {
		// TODO: Detailed log & retry.
		log.Printf("[e] Failed to get subscribed feed sources.\n")
		return nil
	}

	var fs feed.Source
	res := make([]feed.Source, 0, len(srcs))
	for _, src := range srcs {
		// Assume no unmarshalling error.
		json.Unmarshal([]byte(src), &fs)
		res = append(res, fs)
	}
	return res
}

// AppendFeedSubscription tries to add a subscription to a user, return false if already exists (or error).
func AppendFeedSubscription(user string, src feed.Source) bool {
	c := rs.GetConnection()
	defer c.Close()

	userSubKey := util.FormatUserSubsKey(user)

	exists, err := redis.Bool(c.Do("HEXISTS", userSubKey, src.SourceID))
	if err != nil {
		log.Printf("[e] Failed to check whether feed subscription exists for a user.\n")
		return false
	} else if exists {
		return false
	}

	srcPacket, _ := json.Marshal(src)
	if _, err := c.Do("HSET", userSubKey, src.SourceID, srcPacket); err != nil {
		// TODO: Detailed log.
		log.Printf("[e] Failed to append feed subscription to user.\n")
	}
	return true
}

// RemoveFeedSubscription removes the subscribed feed source, return true if successful.
func RemoveFeedSubscription(user, srcID string) bool {
	c := rs.GetConnection()
	defer c.Close()

	userSubKey := util.FormatUserSubsKey(user)
	deleted, err := redis.Int64(c.Do("HDEL", userSubKey, srcID))
	if err != nil {
		// TODO: Detailed log & retry.
		log.Printf("[e] Failed to remove subscription.\n")
		return false
	}
	return deleted == 1
}

// GetUnreadFeedIds returns a list of unread feed IDs.
func GetUnreadFeedIds(user, srcID string) []string {
	c := rs.GetConnection()
	defer c.Close()

	unreadKey := util.FormatUserUnreadKey(user, srcID)
	unreadIds, err := redis.Strings(c.Do("SMEMBERS", unreadKey))
	if err != nil {
		// TODO: Detailed log & retry.
		log.Printf("[e] Failed to get unread feed IDs.\n")
		return nil
	}

	return unreadIds
}

// AppendUnreadFeedItemID adds an unread feed ID to the user w.r.t. a feed source.
func AppendUnreadFeedItemID(user, srcID, feedID string) {
	c := rs.GetConnection()
	defer c.Close()

	unreadKey := util.FormatUserUnreadKey(user, srcID)
	if _, err := c.Do("SADD", unreadKey, feedID); err != nil {
		// TODO: Detailed log & retry.
		log.Printf("[e] Failed to add an unread feed ID.\n")
	}
}

// RemoveUnreadFeedItemID similarly removes an unread feed ID.
func RemoveUnreadFeedItemID(user, srcID, feedID string) {
	c := rs.GetConnection()
	defer c.Close()

	unreadKey := util.FormatUserUnreadKey(user, srcID)
	if _, err := c.Do("SREM", unreadKey, feedID); err != nil {
		// TODO: Detailed log & retry.
		log.Printf("[e] Failed to remove an unread feed ID.\n")
	}
}

// RemoveAllUnreadFeedItem removes all unread feeds.
func RemoveAllUnreadFeedItem(user, srcID string) {
	c := rs.GetConnection()
	defer c.Close()

	unreadKey := util.FormatUserUnreadKey(user, srcID)
	if _, err := c.Do("DEL", unreadKey); err != nil {
		// TODO: Detailed log & retry.
		log.Printf("[e] Failed to remove all unread feeds.\n")
	}
}

// GetUnreadFeedCount returns the count of unread feeds of the user w.r.t. a feed source.
// TODO: Maybe should be done when retrieving subscriptions?
func GetUnreadFeedCount(user, srcID string) int64 {
	c := rs.GetConnection()
	defer c.Close()

	unreadKey := util.FormatUserUnreadKey(user, srcID)
	cnt, err := redis.Int64(c.Do("SCARD", unreadKey))
	if err != nil {
		// TODO: Detailed log & retry.
		log.Printf("[e] Failed to get unread feed count.\n")
	}
	return cnt
}

// InitUserUnreadQueue simply copies feed IDs from the corresponding feed source's latest
// feed streams (which may have duplicates).
func InitUserUnreadQueue(user, srcID string) {
	c := rs.GetConnection()
	defer c.Close()

	unreadKey := util.FormatUserUnreadKey(user, srcID)
	latestFeedIds := feed.GetLatestItemIdsFromSource(srcID)

	if _, err := c.Do("SADD", redis.Args{}.Add(unreadKey).AddFlat(latestFeedIds)...); err != nil {
		// TODO: Detailed log & retry.
		log.Printf("[e] Failed to init unread queue for user.\n")
	}
}
