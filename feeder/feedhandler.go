package feeder

import (
	"crypto/sha1"
	"fmt"
	"log"
	"strconv"
	"sync"
	"time"

	"github.com/edfward/readkey/keyword"
	"github.com/edfward/readkey/model/feed"
	"github.com/edfward/readkey/model/user"
	"github.com/edfward/readkey/util"

	rss "github.com/jteeuwen/go-pkg-rss"
)

type feedHandler struct {
	newSrcCh chan<- feed.FeedSource
	// Keep previous `keepSeenItemNum` feed items.
	seenItems       []string
	keepSeenItemNum int
	channelID       string
	// For random item ID generation.
	channelKey    string
	itemCount     int
	itemCountLock *sync.Mutex
	// Fetcher for keywords or summaries.
	kwFetcher keyword.KeywordFetcher
}

func newFeedHandler(newSrcCh chan<- feed.FeedSource, keywordServerEndPoint string) *feedHandler {
	return &feedHandler{
		newSrcCh:        newSrcCh,
		seenItems:       nil,
		keepSeenItemNum: 50, // Default value.
		channelID:       "",
		channelKey:      "",
		itemCount:       0,
		itemCountLock:   &sync.Mutex{},
		// The keyword server address such as "http://localhost:4567/keywords".
		kwFetcher: keyword.NewKeywordFetcher(keywordServerEndPoint),
	}
}

func (h *feedHandler) ProcessItems(rssFeed *rss.Feed, ch *rss.Channel, items []*rss.Item) {
	// Update capacity. ASSUME it's reasonable.
	// TODO: May not be safe enough.
	if len(items) > h.keepSeenItemNum {
		h.keepSeenItemNum = len(items)
	}

	// Handle channel for first time processing.
	if h.channelID == "" {
		cid := getChannelID(ch)
		// Channel guaranteed not empty.
		h.channelID = cid
		h.channelKey = ch.Key()
	}

	// Get subscribers of the current channel.
	subscribers := feed.GetFeedSourceSubscribers(h.channelID)

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
		feedItem := feed.FeedItem{
			Link:    link,
			Content: *contentPtr,
		}
		feed.SetFeed(id, feedItem)

		// Then append to the feed source's latest queue.
		feed.AppendLatestFeedIdToSource(h.channelID, id)

		// Then append to subscribers' unread queue.
		for _, username := range subscribers {
			user.AppendUnreadFeedId(username, h.channelID, id)
		}

		// Append to its corresponding feed source by spawning a new goroutine.
		lang := getLang(item, ch)
		wg.Add(1)
		go func(id, title, pubDate string) {
			defer wg.Done()
			entry := feed.FeedItemEntry{
				FeedId:  id,
				Title:   title,
				PubDate: pubDate,
			}
			const retry = 3
			fetchResCh := h.kwFetcher.Fetch(contentPtr, lang, retry)
			select {
			case kw := <-fetchResCh:
				entry.Keywords = kw
			// 1 second timeout.
			case <-time.After(1 * time.Second):
			}
			feed.AddFeedEntryToSource(h.channelID, entry)
		}(id, item.Title, item.PubDate)
	}
	wg.Wait()

	// Send back the newly established feed source if haven't done so. Then nullify it.
	if h.newSrcCh != nil {
		h.newSrcCh <- feed.FeedSource{
			URL:      rssFeed.Url,
			SourceId: h.channelID,
			Title:    ch.Title,
		}
		h.newSrcCh = nil
	}
}

func (h *feedHandler) getItemID(i *rss.Item) (res string) {
	// In case of concurrent invocation (which should not happen),
	// it's just for the peace of my mind.
	h.itemCountLock.Lock()
	cnt := h.itemCount
	h.itemCount++
	h.itemCountLock.Unlock()
	// Concatenate the counter to the channel key.
	// TODO: Is channel key really unique?
	key := strconv.Itoa(cnt) + h.channelKey
	res = util.FormatFeedKey(fmt.Sprintf("%x", sha1.Sum([]byte(key))))
	return
}

func getChannelID(ch *rss.Channel) (res string) {
	k := ch.Key()
	res = util.FormatFeedSourceKey(fmt.Sprintf("%x", sha1.Sum([]byte(k))))
	return
}

func getItemContent(i *rss.Item) *string {
	if i.Content != nil {
		// For atom.
		return &i.Content.Text
	}
	// For rss.
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
