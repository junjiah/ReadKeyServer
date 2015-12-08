package api

import (
	"encoding/json"
	"log"

	"github.com/cmu440-F15/paxosapp/libstore"
	"github.com/cmu440-F15/readkey/feeder"
	"github.com/cmu440-F15/readkey/util"
)

// Global services.
var (
	ls libstore.Libstore
	fd feeder.Feeder
)

// Must be called before other functions.
func Setup(ols libstore.Libstore, ofd feeder.Feeder) {
	ls = ols
	fd = ofd
}

func GetSubscribedFeedSources(user string) ([]util.FeedSource, error) {
	userKey := util.FormatUserSubsKey(user)
	srcs, err := ls.GetList(userKey)
	if err != nil {
		// TODO: Proper erorr handling, and possible retries.
		log.Printf("[e] Getting list of subscriptions failed for user %s failed\n", user)
		return nil, err
	}

	var fs util.FeedSource
	res := make([]util.FeedSource, 0, len(srcs))
	for _, src := range srcs {
		// Skip malformed feed sources.
		if err := json.Unmarshal([]byte(src), &fs); err == nil {
			res = append(res, fs)
		}
	}
	return res, nil
}

func Subscribe(user, url string) (*util.FeedSource, error) {
	userKey := util.FormatUserSubsKey(user)
	src, err := fd.GetFeedSource(url)

	if err != nil {
		return nil, err
	}

	// Add the user as the subscriber of the feed source.
	err = ls.AppendToList(util.FormatSubscriberKey(src.SourceId), user)
	if err != nil {
		return nil, err
	}

	srcPacket, _ := json.Marshal(src)
	err = ls.AppendToList(userKey, string(srcPacket))

	if err != nil {
		return nil, err
	}

	// Init unread items for current user.
	unreadKey := util.FormatUserUnreadKey(user, src.SourceId)
	// TODO: Only need to fetch the latest feeds rather than the whole.
	entries, _ := ls.GetList(src.SourceId)
	var fe util.FeedItemEntry
	for _, entry := range entries {
		if err := json.Unmarshal([]byte(entry), &fe); err == nil {
			// TODO: Ignore errors for now.
			ls.AppendToList(unreadKey, fe.FeedId)
		}
	}

	return src, nil
}

func GetFeedItemEntries(user, srcId string, unreadOnly bool) ([]util.FeedItemEntry, error) {
	unreadItems := make(map[string]bool)
	if unreadOnly {
		// TODO: Ignore errors for now.
		unreadIds, _ := ls.GetList(util.FormatUserUnreadKey(user, srcId))
		for _, id := range unreadIds {
			unreadItems[id] = true
		}
	}
	// TODO: Fetching the whole list every time seems inefficient.
	entries, err := ls.GetList(srcId)
	if err != nil {
		return nil, err
	}

	res := make([]util.FeedItemEntry, 0, len(entries))
	var fe util.FeedItemEntry
	for _, entry := range entries {
		// Skip malformed feed item entries.
		if err := json.Unmarshal([]byte(entry), &fe); err != nil {
			continue
		}
		// In unread-only mode, skip already-read items.
		_, unread := unreadItems[fe.FeedId]
		if unreadOnly && !unread {
			continue
		}
		res = append(res, fe)
	}
	return res, nil
}

func GetFeedItem(feedId string) (*util.FeedItem, error) {
	if itemStr, err := ls.Get(feedId); err != nil {
		return nil, err
	} else {
		var item util.FeedItem
		if err = json.Unmarshal([]byte(itemStr), &item); err != nil {
			return nil, err
		} else {
			return &item, nil
		}
	}
}

func MarkFeedItem(user, srcId, feedId string, read bool) (bool, error) {
	var err error
	unreadKey := util.FormatUserUnreadKey(user, srcId)
	if read {
		err = ls.RemoveFromList(unreadKey, feedId)
	} else {
		err = ls.AppendToList(unreadKey, feedId)
	}
	return read, err
}

func GetUnreadCount(user, srcId string) (int, error) {
	unreadIds, err := ls.GetList(util.FormatUserUnreadKey(user, srcId))
	if err != nil {
		return 0, err
	} else {
		return len(unreadIds), nil
	}
}
