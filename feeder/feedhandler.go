package feeder

import (
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/cmu440-F15/paxosapp/libstore"
	"github.com/cmu440-F15/readkey/keyword"
	"github.com/cmu440-F15/readkey/util"
	rss "github.com/jteeuwen/go-pkg-rss"
)

type feedHandler struct {
	ls                 libstore.Libstore
	newChannelNofityCh chan<- channelInfo
	// Keep previous `keepSeenItemNum` feed items.
	seenItems       []string
	keepSeenItemNum int
	channelId       string
	// Fetcher for keywords or summaries.
	kwFetcher keyword.KeywordFetcher
}

func newFeedHandler(ls libstore.Libstore, newChannelNofityCh chan<- channelInfo, keywordServerEndPoint string) *feedHandler {
	return &feedHandler{
		ls:                 ls,
		newChannelNofityCh: newChannelNofityCh,
		seenItems:          nil,
		keepSeenItemNum:    50, // Default value.
		channelId:          "",
		// The keyword server address such as "http://localhost:4567/keywords".
		kwFetcher: keyword.NewKeywordFetcher(keywordServerEndPoint),
	}
}

func (h *feedHandler) ProcessItems(feed *rss.Feed, ch *rss.Channel, items []*rss.Item) {
	// Update capacity. ASSUME it's reasonable.
	// TODO: May not be safe enough.
	if len(items) > h.keepSeenItemNum {
		h.keepSeenItemNum = len(items)
	}

	// Handle channel.
	if h.channelId == "" {
		if cid := getChannelId(ch); cid != "" {
			doneCh := make(chan struct{})
			h.newChannelNofityCh <- channelInfo{feed.Url, cid, ch.Title, doneCh}
			<-doneCh
			h.channelId = cid
		} else {
			// TODO: Proper error handling. Currently simply ignore.
			// The consequence is that: there is a goroutine listening to this channel,
			// but user can never get its feed source ID.
			log.Printf("[e] Processing channel %v failed\n", feed.Url)
			return
		}
	}

	// Get subscribers of the current channel.
	subKey := util.FormatSubscriberKey(h.channelId)
	// TODO: Ignore errors for now.
	subscribers, _ := h.ls.GetList(subKey)

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

	log.Printf("[i] Found %d new items(s) in %s\n", len(newitems), feed.Url)
	var wg sync.WaitGroup
	for _, item := range newitems {
		if id := getItemId(item); id != "" {

			// Append to subscribers' unread queue.
			for _, user := range subscribers {
				unreadKey := util.FormatUserUnreadKey(user, h.channelId)
				// TODO: Error handling.
				h.ls.AppendToList(unreadKey, id)
			}

			// Store the actual content of the feed item.
			contentPtr := getItemContent(item)
			link := ""
			if len(item.Links) > 0 {
				link = item.Links[0].Href
			}
			feedItem := &util.FeedItem{
				Link:    link,
				Content: *contentPtr,
			}
			feedItemPacket, _ := json.Marshal(feedItem)
			// TODO: Error handling.
			h.ls.Put(id, string(feedItemPacket))

			// Append to its corresponding feed source by spawning a new goroutine.
			lang := getLang(item, ch)
			wg.Add(1)
			go func(id, title, pubDate string) {
				defer wg.Done()
				entry := &util.FeedItemEntry{
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
				entryPacket, _ := json.Marshal(entry)
				// TODO: Error handling.
				h.ls.AppendToList(h.channelId, string(entryPacket))
			}(id, item.Title, item.PubDate)
		} else {
			log.Printf("[e] Parsing item ID failed for %v\n", item)
		}
		wg.Wait()
	}
}

func getChannelId(c *rss.Channel) (res string) {
	if c.Id != "" {
		// For atom.
		res = util.FormatFeedSourceKey(c.Id)
	} else if len(c.Links) > 0 {
		// For rss.
		res = util.FormatFeedSourceKey(c.Links[0].Href)
	}
	return
}

func getItemId(i *rss.Item) (res string) {
	if i.Id != "" {
		// For atom.
		res = util.FormatFeedKey(i.Id)
	} else if i.Guid != nil {
		// For rss.
		res = util.FormatFeedKey(*i.Guid)
	} else if len(i.Links) > 0 {
		// Fallback, using links.
		res = util.FormatFeedKey(i.Links[0].Href)
	}
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
