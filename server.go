package main

import (
  "fmt"
  "http"
  "json"
  "os"
  "strings"
  "xml"
)

const (
  REV_HEAD= -1
  REV_FIRST = 0
  LIMIT_NONE = -1
)

type svnClient struct {
  Url string
}

type svnLogItem struct {
  Revision string
  Comment string
  Date string
  Author string
  Paths []string
}

func newSvnLogItem() *svnLogItem {
  i := &svnLogItem{}
  i.Paths = make([]string, 0)
  return i
}

func newSvnClient(url string) *svnClient {
  return &svnClient{url}
}

func charDataString(data xml.Token) string {
  return string([]byte(data.(xml.CharData)))
}

func toSvnLogItems(parser *xml.Parser) ([]*svnLogItem, os.Error) {
  result := make([]*svnLogItem, 0)
  var item *svnLogItem
  for ;; {
    tok, err := parser.Token()
    if err == os.EOF {
      break
    } else if err != nil {
      return nil, err
    }
    switch t := tok.(type) {
    case xml.StartElement:
      switch {
        case t.Name.Local == "log-item":
          item = newSvnLogItem()
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
          item.Revision = charDataString(data)
        case t.Name.Local == "creator-displayname":
          data, err := parser.Token()
          if err != nil {
            return nil, err
          }
          item.Author = charDataString(data)
        case t.Name.Local == "modified-path":
        case t.Name.Local == "added-path":
        case t.Name.Local == "replaced-path":
        case t.Name.Local == "deleted-path":
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

func logRequestPayload(start int, end int, limit int) string {
  p := "<?xml version=\"1.0\"?><S:log-report xmlns:S=\"svn:\">"
  p += fmt.Sprintf("<S:start-revision>%d</S:start-revision><S:end-revision>%d</S:end-revision>", start, end)
  if limit > LIMIT_NONE {
    p += fmt.Sprintf("<S:limit>%d</S:limit>", limit)
  }
  return p + "<S:discover-changed-paths/></S:log-report>"
}

func (l *svnClient) Log(startRev int, endRev int, limit int) ([]*svnLogItem, os.Error) {
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

  return toSvnLogItems(xml.NewParser(res.Body))
}

type svnLogHandler struct {
  client *svnClient
}

func newSvnLogHandler(url string) *svnLogHandler {
  return &svnLogHandler{newSvnClient(url)}
}
func (h *svnLogHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
  w.Header().Set("Content-Type", "application/json;charset=utf-8")
  items, err := h.client.Log(108000, REV_HEAD, LIMIT_NONE)
  if err != nil {
    // TODO: Wrong.
    panic(err)
  }
  json.NewEncoder(w).Encode(items)
}

// TODO:
// 1 - Create http server.
// 2 - Turn logic into talking to svn servers into simple component.
// 3 - Create background poller.
func main() {
  http.Handle("/chrome/", newSvnLogHandler("http://src.chromium.org/svn/trunk"))
  err := http.ListenAndServe(":6565", nil)
  if err != nil {
    panic(err)
  }
}
