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
	return Escape("user-sub:" + user)
}

// Key for mapping from a user + a feed source to its unread feed item entries.
func FormatUserUnreadKey(user, feedSrcId string) string {
	// TODO: Should not allow user to have names containing ':'.
	return Escape("user-unread:" + user + ":" + feedSrcId)
}

// Key for mapping from a feed source to its feed item entries.
func FormatFeedSourceKey(feedSrcId string) string {
	return Escape("src:" + feedSrcId)
}

// Key for mapping from a feed source to its subscribers / users.
func FormatSubscriberKey(feedSrcId string) string {
	return Escape("subscriber:" + feedSrcId)
}

func Escape(s string) string {
	return url.QueryEscape(s)
}
