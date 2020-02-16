package main

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/html"
	"golang.org/x/net/publicsuffix"
)

const (
	baseURL  string = ""
	maxDepth uint16 = 1
)

var (
	lock = sync.RWMutex{}
	wg   = sync.WaitGroup{}
)

var (
	seen map[string]node = map[string]node{}

	transport chan node     = make(chan node)
	signal    chan struct{} = make(chan struct{})
)

var client = http.Client{
	Timeout: 1 * time.Second,
}

type node struct {
	domain string
	parent *node
	depth  uint16
}

func main() {
	var d uint16

	v, _ := parseDomain(baseURL)
	t := node{
		domain: v,
		parent: nil,
		depth:  d,
	}

	start := time.Now()

	wg.Add(1)
	go retrieve(baseURL, d, &t)
	go read()
	wg.Wait()

	signal <- struct{}{}

	fmt.Println("Operation took", time.Since(start))

	writeNodes(seen)
}

func retrieve(u string, d uint16, parent *node) {
	defer wg.Done()

	if d == maxDepth { // max recursive depth reached
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
		case html.ErrorToken: // end of document
			return
		case html.StartTagToken:
			temp := tokenizer.Token()

			isAnchor := temp.Data == "a"

			if isAnchor {
				for _, attribute := range temp.Attr {
					if attribute.Key == "href" {
						domain, err := parseDomain(attribute.Val)

						if err != nil {
							continue
						}

						lock.RLock()
						v, ok := seen[domain] // check if domain already seen
						lock.RUnlock()

						t := node{
							domain: domain,
							parent: parent,
							depth:  d + 1,
						}

						if ok {
							if d+1 < v.depth { // if shorter path, replace depth
								lock.Lock()
								seen[domain] = t
								lock.Unlock()
							}
							continue
						}

						transport <- t
						wg.Add(1)
						go retrieve(attribute.Val, d+1, &t)
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
			seen[v.domain] = v // domain -> {domain, parent, depth}
			lock.Unlock()
		case <-signal:
			return
		}
	}
}

func parseDomain(u string) (string, error) {
	v, err := url.Parse(u)
	if err == nil {
		// ignore non-http[s] schemes
		if v.Scheme == "https" || v.Scheme == "http" {
			h := v.Hostname()
			sep := "."

			// check if hostname is ip address
			p := strings.Join(strings.Split(h, sep), "")

			if _, err := strconv.Atoi(p); err == nil {
				return h, err
			}

			extension, _ := publicsuffix.PublicSuffix(h)
			i := strings.LastIndex(h, extension)
			if i > 1 { // else invalid url hostname format due to parsing
				r := strings.Split(h[:i-1], sep)   // get domain components
				t := r[len(r)-1] + sep + extension // get root domain + extension
				return t, err
			}
			return "", fmt.Errorf("Incorrect URL format")
		}
		return "", fmt.Errorf("Wrong scheme")
	}
	return "", fmt.Errorf("Could not parse domain")
}

func writeNodes(nodes map[string]node) {
	sep := "\n"

	file, _ := os.OpenFile("urls.txt", os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
	defer file.Close()

	for _, v := range nodes {
		t := fmt.Sprintf("%v %v %v", v.domain, v.parent.domain, v.depth) + sep
		file.WriteString(t)
	}
}
