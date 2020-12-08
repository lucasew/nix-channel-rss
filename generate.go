package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/gorilla/feeds"
)

const PERMS = 0755

const indexPageTemplate = `
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Nix channel feed index</title>
    <style>
        body {
            margin: 0;
            padding: 0;
            width: 100vw;
            height: 100vh;
        }
        main {
            display: flex;
            align-items: center;
            justify-content: center;
            flex-direction: column;
            width: inherit;
            height: inherit;
        }
        .items, .item, .icons {
            display: flex;
            flex-direction: row;
        }
        .items {
            width: 60%;
        }
        .item {
            justify-content: space-around;
            align-items: center;
        }
        .icons > a {
            padding: 10px;
        }

    </style>
</head>
<body>
    <main>
        <h1>Nix Channels</h1>
        {{ range . }}
        <section id="items">
            <div class="item">
                <p class="item-name">{{ . }}</p>
                <div class="icons">
                    <a href="{{ . }}/feed.rss">
                        <p>RSS</p>
                    </a>
                    <a href="{{ . }}/feed.atom">
                        <p>ATOM</p>
                    </a>
                    <a href="{{ . }}/feed.json">
                        <p>JSON</p>
                    </a>
                </div>
            </div>
        </section>
        {{ end }}
    </main>
</body>
</html>
`

var (
    channels = []string{
        "nixos-20.03",
        "nixos-20.03-small",
        "nixos-20.09",
        "nixos-20.09-small",
        "nixos-unstable",
        "nixos-unstable-small",
        "nixpkgs-20.03-darwin",
        "nixpkgs-20.09-darwin",
        "nixpkgs-unstable",
    }
    outFolder string
    dumpFeedDatastructures bool
    err error
    wg = sync.WaitGroup{}
)

func init() {
    log.Printf("Setting up...")
    flag.StringVar(&outFolder, "d", "./feeds", "Where to save the channel rss files")
    flag.BoolVar(&dumpFeedDatastructures, "dumpFeedDatastructures", false, "spew.Dump feed datastructures when generated")
    flag.Parse()
    outFolder, err = filepath.Abs(outFolder)
    if err != nil {
        panic(err)
    }
}

type ByDate []*feeds.Item

func (a ByDate) Len() int {
    return len(a)
}

func (a ByDate) Swap(i, j int) {
    a[i], a[j] = a[j], a[i]
}

func (a ByDate) Less(i, j int) bool {
    return a[i].Created.Unix() < a[j].Created.Unix()
}

func main() {
    log.Printf("Starting...")
    wg.Add(len(channels))
    for _, channel := range channels {
        go func(channel string) {
            log.Printf("Generating feed for channel %s", channel)
            rssgen := NewRSSGenerator(channel)
            err := rssgen.WriteFeedsToFolder(outFolder)
            if err != nil {
                log.Printf("Error at channel %s: %s", channel, err.Error())
            }
            wg.Done()
        }(channel)
    }
    wg.Wait()
    tmpl, err := template.New("index").Parse(indexPageTemplate)
    if err != nil {
        panic(err)
    }
    f, err := os.Create(path.Join(outFolder, "index.html"))
    if err != nil {
        panic(err)
    }
    err = tmpl.Execute(f, channels)
    if err != nil {
        panic(err)
    }
}

func httpCat(url string) (string, error) {
    res, err := http.Get(url)
    if err != nil {
        return "", err
    }
    data, err := ioutil.ReadAll(res.Body)
    if err != nil {
        return "", err
    }
    return string(data), nil
}

func trim(s string) string {
    return strings.Trim(s, " \n\r")
}

type HistoryLine struct {
    Commit string
    UnixTimestamp int64
}

func NewRSSGenerator(channel string) *RssGenerator {
    return &RssGenerator{channel}
}

type RssGenerator struct {
    channel string
}

func (r *RssGenerator) WriteFeedsToFolder(folder string) error {
    folder, err = filepath.Abs(path.Join(folder, r.channel))
    if err != nil {
        return err
    }
    err := os.MkdirAll(folder, PERMS)
    if err != nil {
        return err
    }
    feed, err := r.historyToRss()
    if err != nil {
        return err
    }
    rss, err := feed.ToRss()
    if err != nil {
        return err
    }
    atom, err := feed.ToAtom()
    if err != nil {
        return err
    }
    json, err := feed.ToJSON()
    if err != nil {
        return err
    }
    err = ioutil.WriteFile(path.Join(folder, "feed.rss"), []byte(rss), PERMS)
    if err != nil {
        return err
    }
    err = ioutil.WriteFile(path.Join(folder, "feed.atom"), []byte(atom), PERMS)
    if err != nil {
        return err
    }
    return ioutil.WriteFile(path.Join(folder, "feed.json"), []byte(json), PERMS)
}

func (r *RssGenerator) fetchChannelHistory() ([]HistoryLine, error) {
    data, err := httpCat(fmt.Sprintf("https://channels.nix.gsc.io/%s/history", r.channel))
    if err != nil {
        return nil, err
    }
    lines := strings.Split(data, "\n")
    ret := make([]HistoryLine, len(lines))
    for i := 0; i < len(lines); i++ {
        if lines[i] == "" {
            continue
        }
        lineElements := strings.Split(lines[i], " ")
        if len(lineElements) != 2 {
            log.Printf("Skipping invalid line '%s'", lines[i])
            continue
        }
        timestamp, err := strconv.ParseInt(trim(lineElements[1]), 10, 64)
        if err != nil {
            return nil, err
        }
        ret[i] = HistoryLine{trim(lineElements[0]), timestamp}
    }
    return ret, nil
}

func (r *RssGenerator) historyToRss() (feed *feeds.Feed, err error) {
    now := time.Now()
    feed = &feeds.Feed{}
    feed.Title = fmt.Sprintf("Releases for nixpkgs channel %s", r.channel)
    feed.Description = fmt.Sprintf("Feed of all nixpkgs builds for channel %s", r.channel)
    feed.Author = &feeds.Author{Name: "Definitely a machine powered by that blue gopher language", Email: "node-933493@sky.net"}
    feed.Created = now
    feed.Link = &feeds.Link{Href: "localhost:6969"}
    history, err := r.fetchChannelHistory()
    if err != nil {
        return nil, err
    }
    feed.Items = make([]*feeds.Item, 0, len(history))
    unixNow := now.Unix()
    year := int64(3600 * 24 * 365)
    for i := len(history) - 1; i >= 0; i-- {
        item := history[i]
        if (unixNow - int64(item.UnixTimestamp)) > year {
            continue
        }
        itemTime := time.Unix(item.UnixTimestamp, 0)
        feed.Items = append(feed.Items, &feeds.Item{
            Title: fmt.Sprintf("Build %s %s", r.channel, item.Commit),
            Created: itemTime,
            Author: feed.Author,
            Link: &feeds.Link{
                Href: fmt.Sprintf("https://github.com/NixOS/nixpkgs/commit/%s", item.Commit),
            },
            Id: item.Commit,
            Content: fmt.Sprintf("TODO: Fazer uma lista de coisa melhorzinha"),
        })
    }
    if dumpFeedDatastructures {
        spew.Dump(feed)
    }
    sort.Sort(ByDate(feed.Items))
    return feed, nil
}
