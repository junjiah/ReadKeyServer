package feeder

import (
	"errors"
	"log"
	"sync"
	"time"

	"github.com/cmu440-F15/paxosapp/libstore"
	"github.com/cmu440-F15/readkey/util"
	rss "github.com/jteeuwen/go-pkg-rss"
)

// Exported interfaces.
type Feeder interface {
	GetFeedSource(url string) (*util.FeedSource, error)
}

// A struct for feed handlers to send URL-ID pair.
type channelInfo struct {
	url         string
	sourceId    string
	sourceTitle string
	doneCh      chan struct{}
}

type feedSrcInfo struct {
	id     string
	title  string
	active bool
}

// Feed manager.
type feeder struct {
	ls libstore.Libstore
	// Mapping from atom/rss URL to corresponding info.
	urlToFeedSrc          map[string]*feedSrcInfo
	urlToFeedSrcLock      *sync.RWMutex
	newChannelNofityCh    chan channelInfo
	keywordServerEndPoint string
}

// Build the feeder and start the background goroutine.
func NewFeeder(ls libstore.Libstore, keywordServerEndPoint string) *feeder {
	newChannelNofityCh := make(chan channelInfo)
	fd := &feeder{
		ls:                    ls,
		urlToFeedSrc:          make(map[string]*feedSrcInfo),
		urlToFeedSrcLock:      &sync.RWMutex{},
		newChannelNofityCh:    newChannelNofityCh,
		keywordServerEndPoint: keywordServerEndPoint,
	}
	// Start feeder's event handler.
	go fd.run()
	return fd
}

// Event handler.
func (f *feeder) run() {
	for {
		select {
		case c := <-f.newChannelNofityCh:
			f.urlToFeedSrcLock.Lock()
			if info, ok := f.urlToFeedSrc[c.url]; !ok {
				f.urlToFeedSrc[c.url] = &feedSrcInfo{
					id:     c.sourceId,
					title:  c.sourceTitle,
					active: false,
				}
			} else if info.id == "" {
				info.id = c.sourceId
				info.title = c.sourceTitle
			}
			f.urlToFeedSrcLock.Unlock()
			c.doneCh <- struct{}{}
			// TODO: Store to backend storage server using libstore IF NEW,
			// because we need to track what sites are being listened, in case server failure
			// we can re-establish the listening handlers.
		}
	}
}

// Requires the URL to exact match actual feed's URL.
func (f *feeder) GetFeedSource(url string) (*util.FeedSource, error) {
	f.urlToFeedSrcLock.RLock()
	if info, ok := f.urlToFeedSrc[url]; ok && info.id != "" {
		f.urlToFeedSrcLock.RUnlock()
		return &util.FeedSource{info.id, info.title}, nil
	}
	f.urlToFeedSrcLock.RUnlock()

	// Try to listen to the new URL.
	// TODO: Add timeout.
	readyCh := f.storeFeedSource(url)
	select {
	case ready := <-readyCh:
		if !ready {
			return nil, errors.New("fetching feeds error, URL may be invalid")
		}
		// Try to get the ID since it has already fetched something from the URL (in `Fetch`) and
		// invoked item handlers, which passed ID to our event handler and put into the map.
		f.urlToFeedSrcLock.RLock()
		defer f.urlToFeedSrcLock.RUnlock()
		if info, ok := f.urlToFeedSrc[url]; ok && info.id != "" {
			return &util.FeedSource{info.id, info.title}, nil
		} else {
			return nil, errors.New("reading ID error, feed URL may not match exactly, or parsing ID failed")
		}
	}
}

// When encountering a new feed source, begin listening if valid.
func (f *feeder) storeFeedSource(url string) <-chan bool {
	readyCh := make(chan bool)
	// TODO: Change timeout in production.
	const timeout = 1
	handler := newFeedHandler(f.ls, f.newChannelNofityCh, f.keywordServerEndPoint)
	feed := rss.NewWithHandlers(timeout, true, nil, handler)

	go func() {
		// Try to fetch as soon as possible so the caller (usually `GetFeedSourceId`) could get the source ID.
		feed.IgnoreCacheOnce()
		if err := feed.Fetch(url, nil); err != nil {
			// Fetch failed, could be an invalid source. Exit.
			readyCh <- false
			return
		}
		// Fetch succeeded.
		readyCh <- true
		// Try listening to the valid URL.
		f.beginListen(url, feed)
	}()

	return readyCh
}

// Activate a listener goroutine for the URL if not done so.
func (f *feeder) beginListen(url string, feed *rss.Feed) {
	// Lock the map to get the particular feed source lock.
	f.urlToFeedSrcLock.Lock()
	defer f.urlToFeedSrcLock.Unlock()
	// Info should already be created by event handler.
	if info, ok := f.urlToFeedSrc[url]; ok {
		if info.active {
			// It's already been listened. Exit.
			return
		}
		info.active = true
	} else {
		f.urlToFeedSrc[url] = &feedSrcInfo{
			// Empty place holders except the `active` field.
			id:     "",
			title:  "",
			active: true,
		}
	}
	// Spawn a listening goroutine.
	go func() {
		for {
			log.Println("[i] Try fetching")
			if err := feed.Fetch(url, nil); err != nil {
				// TODO: Proper error handling.
				log.Printf("[e] Fetching error %s: %s\n", url, err)
				return
			}

			<-time.After(time.Duration(feed.SecondsTillUpdate() * 1e9))
		}
	}()
}
