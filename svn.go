package svn

import (
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
)

const (
	REV_HEAD   = -1
	REV_FIRST  = 0
	LIMIT_NONE = -1
)

type LogItem struct {
	Revision int64
	Comment  string
	Date     string
	Author   string
	Paths    []string
}

func newLogItem() *LogItem {
	i := &LogItem{}
	i.Paths = make([]string, 0)
	return i
}

func charDataString(data xml.Token) string {
  switch t := data.(type) {
  case xml.CharData:
    return string([]byte(t))
  }
  return ""
}

func toLogItems(parser *xml.Parser) ([]*LogItem, error) {
	result := make([]*LogItem, 0)
	var item *LogItem
	for {
		tok, err := parser.Token()
		if err == io.EOF {
			break
		} else if err != nil {
			return nil, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch {
			case t.Name.Local == "log-item":
				item = newLogItem()
			case t.Name.Local == "comment":
				data, err := parser.Token()
				if err != nil {
					return nil, err
				}
				item.Comment = charDataString(data)
			case t.Name.Local == "date":
				data, err := parser.Token()
				if err != nil {
					return nil, err
				}
				item.Date = charDataString(data)
			case t.Name.Local == "version-name":
				data, err := parser.Token()
				if err != nil {
					return nil, err
				}
				item.Revision, err = strconv.Atoi64(charDataString(data))
				if err != nil {
					return nil, err
				}
			case t.Name.Local == "creator-displayname":
				data, err := parser.Token()
				if err != nil {
					return nil, err
				}
				item.Author = charDataString(data)
			case t.Name.Local == "modified-path",
				t.Name.Local == "added-path",
				t.Name.Local == "replaced-path",
				t.Name.Local == "deleted-path":
				data, err := parser.Token()
				if err != nil {
					return nil, err
				}
				item.Paths = append(item.Paths, charDataString(data))
			}
		case xml.EndElement:
			switch {
			case t.Name.Local == "log-item":
				result = append(result, item)
				item = nil
			}
		}
	}
	return result, nil
}

func logRequestPayload(start int64, end int64, limit int64) string {
	p := "<?xml version=\"1.0\"?><S:log-report xmlns:S=\"svn:\">"
	p += fmt.Sprintf("<S:start-revision>%d</S:start-revision><S:end-revision>%d</S:end-revision>", start, end)
	if limit > LIMIT_NONE {
		p += fmt.Sprintf("<S:limit>%d</S:limit>", limit)
	}
	return p + "<S:discover-changed-paths/></S:log-report>"
}

// A simple Subversion client that supports on limited log generation.
type Client struct {
	Url string
}

func (l *Client) Head() (*LogItem, error) {
  items, err := l.Log(REV_HEAD, REV_HEAD, 1)
  if err != nil {
    return nil, err
  }
  return items[0], nil
}

func (l *Client) Get(path string) (io.ReadCloser, error) {
  c := &http.Client{}
  // todo: what if Url ends with a /?
  r, err := c.Get(fmt.Sprintf("%s%s", l.Url, path))
  if err != nil {
    return nil, err
  }
  return r.Body, nil
}

func (l *Client) Log(startRev int64, endRev int64, limit int64) ([]*LogItem, error) {
	req, err := http.NewRequest(
		"REPORT",
		l.Url,
		strings.NewReader(logRequestPayload(startRev, endRev, limit)))
	if err != nil {
		return nil, err
	}

	c := &http.Client{}
	res, err := c.Do(req)
	if err != nil {
		return nil, err
	}

	return toLogItems(xml.NewParser(res.Body))
}
