package main

import (
  "fmt"
  "http"
  "json"
  "kellegous"
  "strconv"
  "svn"
)

// Model:
// Last N WebKit commits.
// mapping from username => commits.
// last seen commit.

type model struct {
  RecentCommits []*svn.SvnLogItem
  KittenCommits map[string][]int64
  LastKnownRevision int64
}

// An http.Handler that allows JSON access to log data.
type svnLogHandler struct {
  client *svn.SvnClient
}

func newSvnLogHandler(url string) *svnLogHandler {
  return &svnLogHandler{&svn.SvnClient{url}}
}

func stringToInt64(s string, def int64) int64 {
  v, err := strconv.Atoi64(s)
  if err != nil {
    return def
  }
  return v
}

func (h *svnLogHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
  w.Header().Set("Content-Type", "application/json;charset=utf-8")
  startRev := stringToInt64(r.FormValue("s"), svn.REV_HEAD)
  endRev := stringToInt64(r.FormValue("e"), svn.REV_FIRST)
  limit := stringToInt64(r.FormValue("l"), 10)
  items, err := h.client.Log(startRev, endRev, limit)
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
  http.Handle("/", kellegous.NewAppHandler(http.Dir("pub")))

  // Setup /atl
  /*
  http.Handle("/atl/str", websocket.Handler(func(ws *websocket.Conn) {
  })
  */
  fmt.Println("Running...")
  err := http.ListenAndServe(":6565", nil)
  if err != nil {
    panic(err)
  }
}
