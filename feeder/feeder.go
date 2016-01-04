package feeder

import (
	"errors"
	"sync"
	"time"

	"github.com/edfward/readkey/model/feed"

	rss "github.com/jteeuwen/go-pkg-rss"
)

// Feeder is the standard interface to handle feed subscription and retrieval.
type Feeder interface {
	GetFeedSource(url string) (feed.FeedSource, error)
}

// Feed manager.
type feeder struct {
	// Mapping from atom/rss URL to corresponding info.
	urlToFeedSrc          map[string]feed.FeedSource
	urlToFeedSrcLock      *sync.Mutex
	keywordServerEndPoint string
}

// NewFeeder builds the feeder and start the background goroutine.
func NewFeeder(keywordServerEndPoint string) Feeder {
	return &feeder{
		urlToFeedSrc:          make(map[string]feed.FeedSource),
		urlToFeedSrcLock:      &sync.Mutex{},
		keywordServerEndPoint: keywordServerEndPoint,
	}
}

// Requires the URL to exact match actual feed's URL.
func (f *feeder) GetFeedSource(url string) (feed.FeedSource, error) {
	f.urlToFeedSrcLock.Lock()
	defer f.urlToFeedSrcLock.Unlock()

	if src, ok := f.urlToFeedSrc[url]; ok {
		return src, nil
	}

	newSrcCh := make(chan feed.FeedSource)
	// Buffered so the other side will not block even if the current function returns.
	errCh := make(chan error, 1)
	f.listen(url, newSrcCh, errCh)

	select {
	case src := <-newSrcCh:
		f.urlToFeedSrc[src.URL] = src
		return src, nil
	case err := <-errCh:
		return feed.FeedSource{}, errors.New("subscribe faild: " + err.Error())
	}
}

// When encountering a new feed source, begin listening if valid.
func (f *feeder) listen(url string, newSrcCh chan<- feed.FeedSource, errCh chan<- error) {
	// TODO: Change timeout in production. For now it's 1 min.
	const timeout = 1
	handler := newFeedHandler(newSrcCh, f.keywordServerEndPoint)
	rssFeed := rss.NewWithHandlers(timeout, true, nil, handler)
	rssFeed.IgnoreCacheOnce()

	go func() {
		for {
			if err := rssFeed.Fetch(url, nil); err != nil {
				// Fetch failed, could be an invalid source. Exit. Will not block since it's buffered.
				errCh <- err
				return
			}
			<-time.After(time.Duration(rssFeed.SecondsTillUpdate() * 1e9))
		}
	}()
}
