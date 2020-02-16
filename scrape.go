package main

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/html"
	"golang.org/x/net/publicsuffix"
)

const (
	baseURL  string = ""
	maxDepth uint8  = 0
)

var (
	lock = sync.RWMutex{}
	wg   = sync.WaitGroup{}
)

var (
	seen map[string]struct{} = map[string]struct{}{}

	transport chan string   = make(chan string)
	signal    chan struct{} = make(chan struct{})
)

var client = http.Client{
	Timeout: 1 * time.Second,
}

func main() {
	var d uint8

	start := time.Now()

	wg.Add(1)
	go retrieve(baseURL, d)
	go read()
	wg.Wait()

	signal <- struct{}{}

	fmt.Println("Operation took", time.Since(start))

	sep := "\n"

	file, _ := os.OpenFile("urls.txt", os.O_APPEND|os.O_WRONLY, 0644)
	defer file.Close()

	for k := range seen {
		file.WriteString(k + sep)
	}
}

func retrieve(u string, d uint8) {
	defer wg.Done()

	if d == maxDepth {
		return
	}

	response, err := client.Get(u)

	if err != nil {
		return
	}

	tokenizer := html.NewTokenizer(response.Body)
	defer response.Body.Close()

	for {
		token := tokenizer.Next()

		switch token {
		case html.ErrorToken:
			return
		case html.StartTagToken:
			temp := tokenizer.Token()

			isAnchor := temp.Data == "a"

			if isAnchor {
				for _, attribute := range temp.Attr {
					if attribute.Key == "href" {
						dom, err := parseDomain(attribute.Val)

						if err != nil {
							continue
						}

						lock.RLock()
						_, ok := seen[dom]
						lock.RUnlock()

						if !ok {
							transport <- dom
							wg.Add(1)
							go retrieve(attribute.Val, d+1)
						}
					}
				}
			}
		}
	}
}

func read() {
	for {
		select {
		case v := <-transport:
			lock.Lock()
			seen[v] = struct{}{}
			lock.Unlock()
		case <-signal:
			return
		}
	}
}

func parseDomain(u string) (string, error) {
	v, err := url.Parse(u)
	if err == nil {
		if v.Scheme == "https" || v.Scheme == "http" {
			h := v.Hostname()
			sep := "."

			extension, _ := publicsuffix.PublicSuffix(h)
			i := strings.LastIndex(h, extension)
			if i > 1 {
				r := strings.Split(h[:i-1], sep)
				t := r[len(r)-1] + sep + extension
				return t, err
			}
			return "", fmt.Errorf("Incorrect URL format")
		}
		return "", fmt.Errorf("Wrong scheme")
	}
	return "", fmt.Errorf("Could not parse domain")
}
