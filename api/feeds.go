package handler

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	// "github.com/davecgh/go-spew/spew"
	"github.com/gorilla/feeds"
)

var (
    channels = []string{
        "nixos-22.05",
        "nixos-22.05-small",
        "nixos-22.11",
        "nixos-22.11-small",
        "nixos-unstable",
        "nixos-unstable-small",
        "nixpkgs-22.11-darwin",
        "nixpkgs-22.11-darwin",
        "nixpkgs-unstable",
    }
)

func CheckIfChannelExist(channel string) bool {
    for i := 0; i < len(channels); i++ {
        if channels[i] == channel {
            return true
        }
    }
    return false
}

type ByDate []*feeds.Item

func (a ByDate) Len() int {
    return len(a)
}

type HistoryLine struct {
    Commit string
    UnixTimestamp int64
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

func fetchChannelHistory(channel string) ([]HistoryLine, error) {
    data, err := httpCat(fmt.Sprintf("https://channels.nix.gsc.io/%s/history", channel))
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

func generateRSSFromChannel(channel string) (feed *feeds.Feed, err error) {
    now := time.Now()
    feed = &feeds.Feed{}
    feed.Title = fmt.Sprintf("Releases for nixpkgs channel %s", channel)
    feed.Description = fmt.Sprintf("Feed of all nixpkgs builds for channel %s", channel)
    feed.Author = &feeds.Author{Name: "Definitely a machine powered by that blue gopher language", Email: "node-933493@sky.net"}
    feed.Created = now
    feed.Link = &feeds.Link{Href: "localhost:6969"}
    history, err := fetchChannelHistory(channel)
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
        feed.Items = append(feed.Items, &feeds.Item{
            Title: fmt.Sprintf("Build %s %s", channel, item.Commit),
            Created: time.Unix(item.UnixTimestamp, 0),
            Author: feed.Author,
            Link: &feeds.Link{
                Href: fmt.Sprintf("https://github.com/NixOS/nixpkgs/commit/%s", item.Commit),
            },
            Id: item.Commit,
            Content: fmt.Sprintf("TODO: Fazer uma lista de coisa melhorzinha"),
        })
    }
    sort.Sort(ByDate(feed.Items))
    return feed, nil
}

func (a ByDate) Swap(i, j int) {
    a[i], a[j] = a[j], a[i]
}

func (a ByDate) Less(i, j int) bool {
    return a[i].Created.Unix() > a[j].Created.Unix()
}

func Handler(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Cache-Control", "public, max-age=3600")
    channel := r.URL.Query().Get("channel")
    format := r.URL.Query().Get("format")
    log.Printf("Requested channel %s at format %s", channel, format)
    rss, err := generateRSSFromChannel(channel)
    if err != nil {
        w.WriteHeader(400)
        w.Write([]byte(err.Error()))
        return
    }
    switch (format) {
        case "rss":
            w.Header().Set("Content-Type", "application/rss+xml")
            r, err := rss.ToRss()
            if err != nil {
                w.WriteHeader(500)
                fmt.Fprint(w, err.Error())
            }
            fmt.Fprint(w, r)
        case "atom":
            w.Header().Set("Content-Type", "application/atom+xml")
            r, err := rss.ToAtom()
            if err != nil {
                w.WriteHeader(500)
                fmt.Fprint(w, err.Error())
            }
            fmt.Fprint(w, r)

        case "json":
            w.Header().Set("Content-Type", "application/json")
            r, err := rss.ToJSON()
            if err != nil {
                w.WriteHeader(500)
                fmt.Fprint(w, err.Error())
            }
            fmt.Fprint(w, r)
        default:
            w.WriteHeader(404)
            fmt.Fprintf(w, "no such format: %s", format)
    }
    return
}
