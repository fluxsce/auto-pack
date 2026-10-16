package oss

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

const listMaxKeys = 100

// Entry 是桶里的一个对象或一层目录。
type Entry struct {
	Key          string
	Name         string
	Dir          bool
	Size         int64
	LastModified string
}

// Page 是一次按前缀列出的结果。
type Page struct {
	Prefix     string
	NextMarker string
	Truncated  bool
	Entries    []Entry
}

// List 按前缀列出对象。delimiter 固定为 /，目录与文件分开返回。
func List(ctx context.Context, cfg Config, prefix, marker string) (Page, error) {
	req, err := newList(cfg, prefix, marker, time.Now().UTC())
	if err != nil {
		return Page{}, err
	}
	if ctx != nil {
		req = req.WithContext(ctx)
	}
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return Page{}, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if resp.StatusCode/100 != 2 {
		msg := strings.TrimSpace(string(body))
		if msg == "" {
			msg = resp.Status
		}
		return Page{}, fmt.Errorf("oss list %s: %s", resp.Status, msg)
	}
	return parseList(prefix, body)
}

func newList(cfg Config, prefix, marker string, now time.Time) (*http.Request, error) {
	host, https, err := endpointHost(cfg)
	if err != nil {
		return nil, err
	}
	pairs := [][2]string{{"delimiter", "/"}, {"max-keys", strconv.Itoa(listMaxKeys)}}
	if marker != "" {
		pairs = append(pairs, [2]string{"marker", marker})
	}
	if prefix != "" {
		pairs = append(pairs, [2]string{"prefix", prefix})
	}
	sort.Slice(pairs, func(i, j int) bool { return pairs[i][0] < pairs[j][0] })
	canon := "/" + cfg.Bucket + "/?"
	q := url.Values{}
	for i, pair := range pairs {
		if i > 0 {
			canon += "&"
		}
		canon += pair[0] + "=" + pair[1]
		q.Set(pair[0], pair[1])
	}
	scheme := "https"
	if !https {
		scheme = "http"
	}
	req, err := http.NewRequest(http.MethodGet, scheme+"://"+cfg.Bucket+"."+host+"/?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	date := now.Format(http.TimeFormat)
	req.Header.Set("Date", date)
	headerCanon := ""
	if token := strings.TrimSpace(cfg.SecurityToken); token != "" {
		req.Header.Set("x-oss-security-token", token)
		headerCanon = "x-oss-security-token:" + token + "\n"
	}
	stringToSign := "GET\n\n\n" + date + "\n" + headerCanon + canon
	req.Header.Set("Authorization", "OSS "+cfg.AccessKeyID+":"+sign(cfg.AccessKeySecret, stringToSign))
	return req, nil
}

type listXML struct {
	IsTruncated    bool          `xml:"IsTruncated"`
	NextMarker     string        `xml:"NextMarker"`
	Contents       []listContent `xml:"Contents"`
	CommonPrefixes []listPrefix  `xml:"CommonPrefixes"`
}

type listContent struct {
	Key          string `xml:"Key"`
	Size         int64  `xml:"Size"`
	LastModified string `xml:"LastModified"`
}

type listPrefix struct {
	Prefix string `xml:"Prefix"`
}

func parseList(prefix string, body []byte) (Page, error) {
	var doc listXML
	if err := xml.Unmarshal(body, &doc); err != nil {
		return Page{}, err
	}
	page := Page{Prefix: prefix, NextMarker: doc.NextMarker, Truncated: doc.IsTruncated}
	for _, dir := range doc.CommonPrefixes {
		p := strings.TrimSpace(dir.Prefix)
		if p == "" {
			continue
		}
		page.Entries = append(page.Entries, Entry{Key: p, Name: displayName(prefix, p), Dir: true})
	}
	for _, obj := range doc.Contents {
		key := strings.TrimSpace(obj.Key)
		if key == "" || key == prefix {
			continue
		}
		page.Entries = append(page.Entries, Entry{
			Key: key, Name: displayName(prefix, key), Size: obj.Size, LastModified: obj.LastModified,
		})
	}
	return page, nil
}

func displayName(prefix, key string) string {
	name := strings.TrimPrefix(key, prefix)
	return strings.Trim(name, "/")
}
