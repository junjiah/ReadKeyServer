package util

import (
	"net/url"
)

// Key for mapping from a feed to its actual content.
func FormatFeedKey(feedId string) string {
	return Escape("feed:" + feedId)
}

// Key for mapping from a user to his subscribed feed sources.
func FormatUserSubsKey(user string) string {
	// TODO: Should be like 'user:1000:sub'.
	return Escape("subs:" + user)
}

// Key for mapping from a user + a feed source to its unread feed IDs.
func FormatUserUnreadKey(user, feedSrcId string) string {
	// TODO: Should not allow user to have names containing ':',
	// rather, it should be 'user:1000:src:xyt.com:unread'.
	return Escape("unread:" + user + ":" + feedSrcId)
}

// Key for mapping from a feed source to its feed item entries.
func FormatFeedSourceKey(rawFeedSrcId string) string {
	return Escape("src:" + rawFeedSrcId)
}

// Key for mapping from a feed source to its latest feed IDs.
func FormatLatestFeedsKey(feedSrcId string) string {
	return Escape("latest:" + feedSrcId)
}

// Key for mapping from a feed source to its subscribers / users.
func FormatSubscriberKey(feedSrcId string) string {
	return Escape("subscriber:" + feedSrcId)
}

// Key to retrieve currently listening feed sources.
func FormatListeningKey() string {
	return "listening"
}

func Escape(s string) string {
	return url.QueryEscape(s)
}
