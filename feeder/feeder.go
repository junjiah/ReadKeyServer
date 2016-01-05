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
	GetFeedSource(url string) (feed.Source, error)
}

// Feed manager.
type feeder struct {
	// Mapping from atom/rss URL to corresponding info.
	urlToFeedSrc          map[string]feed.Source
	urlToFeedSrcLock      *sync.Mutex
	keywordServerEndPoint string
}

// NewFeeder builds the feeder and start the background goroutine.
func NewFeeder(keywordServerEndPoint string) Feeder {
	fd := &feeder{
		urlToFeedSrc:          make(map[string]feed.Source),
		urlToFeedSrcLock:      &sync.Mutex{},
		keywordServerEndPoint: keywordServerEndPoint,
	}
	// Try to re-listen to feed sources if existing, as a recovery method.
	fd.recover()
	return fd
}

// Requires the URL to exact match actual feed's URL.
func (f *feeder) GetFeedSource(url string) (feed.Source, error) {
	f.urlToFeedSrcLock.Lock()
	defer f.urlToFeedSrcLock.Unlock()

	if src, ok := f.urlToFeedSrc[url]; ok {
		return src, nil
	}

	newSrcCh := make(chan feed.Source)
	// Buffered so the other side will not block even if the current function returns.
	errCh := make(chan error, 1)
	f.listen(url, newSrcCh, errCh)

	select {
	case src := <-newSrcCh:
		f.urlToFeedSrc[src.URL] = src
		feed.AppendListeningSource(src)
		return src, nil
	case err := <-errCh:
		return feed.Source{}, errors.New("subscribe failed: " + err.Error())
	}
}

// When encountering a new feed source, begin listening if valid.
func (f *feeder) listen(url string, newSrcCh chan<- feed.Source, errCh chan<- error) {
	const timeout = 5
	handler := newFeedHandler(newSrcCh, f.keywordServerEndPoint)
	rssFeed := rss.NewWithHandlers(timeout, true, nil, handler)
	rssFeed.IgnoreCacheOnce()

	go func() {
		for {
			if err := rssFeed.Fetch(url, nil); err != nil {
				// Fetch failed, could be an invalid source. Exit.
				if errCh != nil {
					// Will not block since it's buffered.
					errCh <- err
				}
				return
			}
			<-time.After(time.Duration(rssFeed.SecondsTillUpdate() * 1e9))
		}
	}()
}

// Recovery happens when starting after the server accidentally exits. Primarily it reconstructs the URL to
// feed source map and restarts listening to them. However there are some unexpected consequences:
// 1. Every feed item will be regarded as new to subscribed users. (Hopefully acceptable.)
// 2. In this way, latest feeds for each source may contain duplicate items. (Also hopefully acceptable.)
func (f *feeder) recover() {
	listeningSrcs := feed.GetListeningSources()
	for _, src := range listeningSrcs {
		f.urlToFeedSrc[src.URL] = src
		f.listen(src.URL, nil, nil)
	}
}
