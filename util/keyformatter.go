package util

import (
	"net/url"
)

// FormatFeedKey returns key for mapping from a feed to its actual content.
func FormatFeedKey(feedID string) string {
	return Escape("feed:" + feedID)
}

// FormatUserSubsKey returns key for mapping from a user to his subscribed feed sources.
func FormatUserSubsKey(user string) string {
	return Escape("subs:" + user)
}

// FormatUserUnreadKey returns key for mapping from a user + a feed source to its unread feed IDs.
func FormatUserUnreadKey(user, feedSrcID string) string {
	return Escape("unread:" + user + ":" + feedSrcID)
}

// FormatFeedSourceKey returns key for mapping from a feed source to its feed item entries.
func FormatFeedSourceKey(rawFeedSrcID string) string {
	return Escape("src:" + rawFeedSrcID)
}

// FormatLatestFeedsKey returns key for mapping from a feed source to its latest feed IDs.
func FormatLatestFeedsKey(feedSrcID string) string {
	return Escape("latest:" + feedSrcID)
}

// FormatSubscriberKey returns key for mapping from a feed source to its subscribers / users.
func FormatSubscriberKey(feedSrcID string) string {
	return Escape("subscriber:" + feedSrcID)
}

// FormatListeningKey returns key to retrieve currently listening feed sources.
func FormatListeningKey() string {
	return "listening"
}

// Escape simply used `QueryEscape` from `url` library.
func Escape(s string) string {
	return url.QueryEscape(s)
}
