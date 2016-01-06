package feeder

import (
	"crypto/sha1"
	"fmt"
	"html"
	"log"
	"regexp"
	"sync"
	"time"

	"github.com/edfward/readkey/keyword"
	"github.com/edfward/readkey/model/feed"
	"github.com/edfward/readkey/model/user"
	"github.com/edfward/readkey/util"

	rss "github.com/jteeuwen/go-pkg-rss"
)

// A utility regular expression pattern to find RSS feed content using RSS1.0 Content Module Specification.
var contentRe = regexp.MustCompile("^<[^>]*http://purl.org/rss/1.0/modules/content[^>]*>(.*)<[^>]*http://purl.org/rss/1.0/modules/content[^>]*>$")

type feedHandler struct {
	newSrcCh chan<- feed.Source
	// Keep previous `keepSeenItemNum` feed items.
	seenItems       []string
	keepSeenItemNum int
	channelID       string
	channelURL      string
	// Fetcher for keywords or summaries.
	kwFetcher keyword.Fetcher
}

func newFeedHandler(newSrcCh chan<- feed.Source, keywordServerEndPoint string) *feedHandler {
	return &feedHandler{
		newSrcCh:        newSrcCh,
		seenItems:       nil,
		keepSeenItemNum: 50, // Default value.
		channelURL:      "", // Canonical URL acquired in `ProcessItems`.
		channelID:       "", // Hash of the channel URL.
		// The keyword server address such as "http://localhost:4567/keywords".
		kwFetcher: keyword.NewKeywordFetcher(keywordServerEndPoint),
	}
}

func (h *feedHandler) ProcessItems(rssFeed *rss.Feed, ch *rss.Channel, items []*rss.Item) {
	// Update capacity. ASSUME it's reasonable.
	if len(items) > h.keepSeenItemNum {
		h.keepSeenItemNum = len(items)
	}

	// Handle channel for first time processing. Using hash of URL as the unique ID.
	if h.channelID == "" {
		h.channelURL = rssFeed.Url
		h.channelID = getChannelID(h.channelURL)
	}

	// Get subscribers of the current channel.
	subscribers := feed.GetSourceSubscribers(h.channelID)

	// Handle items.
	var newitems []*rss.Item
	for _, v := range items {
		if k := v.Key(); !contains(h.seenItems, k) {
			newitems = append(newitems, v)
			h.seenItems = append(h.seenItems, k)
		}
	}
	// Truncate previously seen items if exceeds capacity.
	if len(h.seenItems) > h.keepSeenItemNum {
		h.seenItems = h.seenItems[len(h.seenItems)-h.keepSeenItemNum:]
	}

	log.Printf("[i] Found %d new items(s) in %s\n", len(newitems), rssFeed.Url)
	var wg sync.WaitGroup
	for _, item := range newitems {

		id := h.getItemID(item)

		// Store the actual content of the feed item.
		contentPtr := getItemContent(item)
		link := ""
		if len(item.Links) > 0 {
			link = item.Links[0].Href
		}
		feedItem := feed.Item{
			Link:    link,
			Content: *contentPtr,
		}
		feed.SetItem(id, feedItem)

		// Then append to the feed source's latest queue.
		feed.AppendLatestItemIDToSource(h.channelID, id)

		// Then append to subscribers' unread queue.
		for _, username := range subscribers {
			user.AppendUnreadFeedItemID(username, h.channelID, id)
		}

		// Append to its corresponding feed source by spawning a new goroutine.
		lang := getLang(item, ch)
		wg.Add(1)
		go func(id, title, pubDate string) {
			defer wg.Done()
			entry := feed.ItemEntry{
				FeedID:  id,
				Title:   title,
				PubDate: pubDate,
			}
			const retry = 3
			fetchResCh := h.kwFetcher.Fetch(contentPtr, lang, retry)
			select {
			case kw := <-fetchResCh:
				entry.Keywords = kw
			// 10 seconds timeout.
			case <-time.After(10 * time.Second):
			}
			feed.AddItemEntryToSource(h.channelID, entry)
		}(id, item.Title, item.PubDate)
	}
	wg.Wait()

	// Send back the newly established feed source if haven't done so. Then nullify the channel.
	if h.newSrcCh != nil {
		h.newSrcCh <- feed.Source{
			URL:      rssFeed.Url,
			SourceID: h.channelID,
			Title:    ch.Title,
		}
		h.newSrcCh = nil
	}
}

func (h *feedHandler) getItemID(i *rss.Item) (res string) {
	// Based on channel key, then concatenate the per-item ID.
	itemID := h.channelURL
	if i.Id != "" { // Atom.
		itemID += i.Id
	} else if i.Guid != nil { // RSS.
		itemID += *i.Guid
	} else {
		// Fallback, use required title + description. Should happen rarely.
		itemID += i.Title + *getItemContent(i)
	}
	return util.FormatFeedKey(fmt.Sprintf("%x", sha1.Sum([]byte(itemID))))
}

func getChannelID(key string) string {
	return util.FormatFeedSourceKey(fmt.Sprintf("%x", sha1.Sum([]byte(key))))
}

func getItemContent(i *rss.Item) *string {
	if i.Content != nil {
		if match := contentRe.FindStringSubmatch(i.Content.Text); match != nil {
			// The feed uses RSS1.0 content modules, needs HTML unescape.
			// Ref: https://github.com/jteeuwen/go-pkg-rss/blob/31df4852b3d06032f59342013e6362526be0cd72/rss.go#L199
			unescaped := html.UnescapeString(match[1])
			return &unescaped
		}
		return &i.Content.Text
	}
	// Fallback. For normal RSS.
	return &i.Description
}

func getLang(i *rss.Item, ch *rss.Channel) (res string) {
	if i.Content != nil {
		res = i.Content.Lang
	} else {
		res = ch.Language
	}
	return
}

func contains(ls []string, val string) bool {
	for _, v := range ls {
		if val == v {
			return true
		}
	}
	return false
}
