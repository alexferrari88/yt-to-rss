package service

import (
	"crypto/subtle"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Command struct{ Action, Argument string }
type Response struct {
	Result any    `json:"result,omitempty"`
	Error  string `json:"error,omitempty"`
}

func (s *Service) command(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != "POST" {
		w.WriteHeader(405)
		return
	}
	var c Command
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 16384)).Decode(&c) != nil {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(Response{Error: "invalid command"})
		return
	}
	var result any
	var err error
	switch c.Action {
	case "add":
		result, err = s.Add(r.Context(), c.Argument)
	case "list":
		result, err = s.List(r.Context())
	case "retry":
		result, err = s.Retry(r.Context(), c.Argument)
	case "delete":
		result, err = s.Delete(r.Context(), c.Argument)
	case "status":
		if c.Argument == "" {
			result, err = s.List(r.Context())
		} else {
			result, err = s.Status(r.Context(), c.Argument)
		}
	case "feed-url":
		result = s.cfg.BaseURL + "/" + s.cfg.ReadToken + "/feed.xml"
	default:
		err = fmt.Errorf("unknown command")
	}
	if err != nil {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(Response{Error: err.Error()})
		return
	}
	json.NewEncoder(w).Encode(Response{Result: result})
}

type rss struct {
	XMLName xml.Name `xml:"rss"`
	Version string   `xml:"version,attr"`
	ITunes  string   `xml:"xmlns:itunes,attr"`
	Channel channel  `xml:"channel"`
}
type channel struct {
	Title       string `xml:"title"`
	Description string `xml:"description"`
	Link        string `xml:"link"`
	Items       []item `xml:"item"`
}
type guid struct {
	Value       string `xml:",chardata"`
	IsPermaLink string `xml:"isPermaLink,attr"`
}
type enclosure struct {
	URL    string `xml:"url,attr"`
	Length int64  `xml:"length,attr"`
	Type   string `xml:"type,attr"`
}
type item struct {
	Title       string    `xml:"title"`
	Description string    `xml:"description"`
	Link        string    `xml:"link"`
	GUID        guid      `xml:"guid"`
	PubDate     string    `xml:"pubDate"`
	Author      string    `xml:"itunes:author,omitempty"`
	Duration    string    `xml:"itunes:duration,omitempty"`
	Enclosure   enclosure `xml:"enclosure"`
}

func (s *Service) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" && r.Method != "HEAD" {
		http.NotFound(w, r)
		return
	}
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/"), "/")
	if len(parts) < 2 || subtle.ConstantTimeCompare([]byte(parts[0]), []byte(s.cfg.ReadToken)) != 1 {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Cache-Control", "private, no-cache")
	w.Header().Set("Referrer-Policy", "no-referrer")
	if len(parts) == 2 && parts[1] == "feed.xml" {
		s.feed(w, r)
		return
	}
	if len(parts) == 3 && parts[1] == "media" && strings.HasSuffix(parts[2], ".mp3") {
		id := strings.TrimSuffix(parts[2], ".mp3")
		s.mu.Lock()
		v, err := s.status(r.Context(), id)
		if err != nil || v.State != "published" || !v.PublishedAt.Add(s.cfg.Retention).After(time.Now()) {
			s.mu.Unlock()
			http.NotFound(w, r)
			return
		}
		f, err := os.Open(filepath.Join(s.cfg.StateDir, "media", v.ID+".mp3"))
		s.mu.Unlock()
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defer f.Close()
		info, err := f.Stat()
		if err != nil || !info.Mode().IsRegular() {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "audio/mpeg")
		http.ServeContent(w, r, v.ID+".mp3", *v.PublishedAt, f)
		return
	}
	http.NotFound(w, r)
}
func (s *Service) feed(w http.ResponseWriter, r *http.Request) {
	submissions, err := s.List(r.Context())
	if err != nil {
		http.Error(w, "feed temporarily unavailable", 503)
		return
	}
	doc := rss{Version: "2.0", ITunes: "http://www.itunes.com/dtds/podcast-1.0.dtd", Channel: channel{Title: s.cfg.FeedTitle, Description: s.cfg.FeedDescription, Link: s.cfg.FeedLink}}
	for _, v := range submissions {
		if v.State != "published" || !v.PublishedAt.Add(s.cfg.Retention).After(time.Now()) {
			continue
		}
		doc.Channel.Items = append(doc.Channel.Items, item{Title: v.Title, Description: fmt.Sprintf("%s\nSource: %s", v.Uploader, v.SourceURL), Link: v.SourceURL, GUID: guid{Value: "youtube:" + v.ID, IsPermaLink: "false"}, PubDate: v.PublishedAt.Format(time.RFC1123Z), Author: v.Uploader, Duration: fmt.Sprintf("%.0f", v.Duration), Enclosure: enclosure{URL: s.cfg.BaseURL + "/" + s.cfg.ReadToken + "/media/" + v.ID + ".mp3", Length: v.Bytes, Type: "audio/mpeg"}})
	}
	b, err := xml.Marshal(doc)
	if err != nil {
		http.Error(w, "feed temporarily unavailable", 503)
		return
	}
	w.Header().Set("Content-Type", "application/rss+xml; charset=utf-8")
	w.Header().Set("Content-Length", fmt.Sprint(len(xml.Header)+len(b)))
	if r.Method == "GET" {
		w.Write([]byte(xml.Header))
		w.Write(b)
	}
}
