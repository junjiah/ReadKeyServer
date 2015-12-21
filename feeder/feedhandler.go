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
	newChannelNofityCh chan<- channelInfo
	// Keep previous `keepSeenItemNum` feed items.
	seenItems       []string
	keepSeenItemNum int
	channelId       string
	// For random item ID generation.
	channelKey    string
	itemCount     int
	itemCountLock *sync.Mutex
	// Fetcher for keywords or summaries.
	kwFetcher keyword.KeywordFetcher
}

func newFeedHandler(newChannelNofityCh chan<- channelInfo, keywordServerEndPoint string) *feedHandler {
	return &feedHandler{
		newChannelNofityCh: newChannelNofityCh,
		seenItems:          nil,
		keepSeenItemNum:    50, // Default value.
		channelId:          "",
		channelKey:         "",
		itemCount:          0,
		itemCountLock:      &sync.Mutex{},
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

	// Handle channel.
	if h.channelId == "" {
		cid := getChannelId(ch)
		doneCh := make(chan struct{})
		h.newChannelNofityCh <- channelInfo{rssFeed.Url, cid, ch.Title, doneCh}
		<-doneCh
		h.channelId = cid
		h.channelKey = ch.Key()
	}

	// Get subscribers of the current channel.
	subscribers := feed.GetFeedSourceSubscribers(h.channelId)

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

		id := h.getItemId(item)

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
		feed.AppendLatestFeedIdToSource(h.channelId, id)

		// Then append to subscribers' unread queue.
		for _, username := range subscribers {
			user.AppendUnreadFeedId(username, h.channelId, id)
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
			feed.AddFeedEntryToSource(h.channelId, entry)
		}(id, item.Title, item.PubDate)
	}
	wg.Wait()
}

func (h *feedHandler) getItemId(i *rss.Item) (res string) {
	// In case of concurrent invocation (which should not happen),
	// it's just for the peace of my mind.
	h.itemCountLock.Lock()
	cnt := h.itemCount
	h.itemCount++
	h.itemCountLock.Unlock()
	// Concatenate the counter to the channel key.
	key := strconv.Itoa(cnt) + h.channelKey
	res = util.FormatFeedKey(fmt.Sprintf("%x", sha1.Sum([]byte(key))))
	return
}

func getChannelId(ch *rss.Channel) (res string) {
	k := ch.Key()
	res = util.FormatFeedSourceKey(fmt.Sprintf("%x", sha1.Sum([]byte(k))))
	return
}

func getItemContent(i *rss.Item) *string {
	if i.Content != nil {
		// For atom.
		return &i.Content.Text
	} else {
		// For rss.
		return &i.Description
	}
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
