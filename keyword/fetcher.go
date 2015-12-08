package keyword

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"

	"github.com/kennygrant/sanitize"
)

type KeywordFetcher interface {
	Fetch(contentPtr *string, lang string, retry int) <-chan string
}

type keywordFetcher struct {
	serverAddr string
}

// A special dummy fetcher, mostly for testing.
type summaryFetcher struct {
}

func NewKeywordFetcher(serverAddr string) *keywordFetcher {
	return &keywordFetcher{
		serverAddr: serverAddr,
	}
}

func NewSummaryFetcher() *summaryFetcher {
	return &summaryFetcher{}
}

// Simply retrieve a small part of the content, mostly for testing.
func (f *summaryFetcher) Fetch(contentPtr *string, lang string, retry int) <-chan string {
	// `retry` and `lang` are redundant for summary extraction.
	resCh := make(chan string, 1)
	const summarySize = 30
	go func() {
		summary := sanitize.HTML(*contentPtr)
		// Handle Unicode.
		r := []rune(summary)
		// Truncate if too many.
		if len(r) > summarySize {
			r = r[:summarySize]
		}
		resCh <- string(r)
	}()
	return resCh
}

// Fetch keywords by sending requests to the keyword server.
func (f *keywordFetcher) Fetch(contentPtr *string, lang string, retry int) <-chan string {
	resCh := make(chan string, 1)
	go func() {
		formData := url.Values{
			"size": {"5"},  // Number of keywords.
			"lang": {lang}, // Could be empty, if so let the keyword server decide.
			"text": {sanitize.HTML(*contentPtr)},
		}
		for i := 0; i < retry; i++ {
			resp, err := http.PostForm(f.serverAddr, formData)
			if err != nil {
				continue
			}

			// Successful request.
			defer resp.Body.Close()
			body, _ := ioutil.ReadAll(resp.Body)

			var respJson struct {
				Keywords []string `json: "keywords"`
			}

			if err := json.Unmarshal(body, &respJson); err != nil {
				// TODO: Ignore if parsing json fails.
			} else {
				resCh <- strings.Join(respJson.Keywords, ",")
			}

			// Exit the loop when successful.
			break
		}
	}()
	return resCh
}
