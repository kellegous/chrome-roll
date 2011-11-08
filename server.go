package main

import (
	"fmt"
	"http"
	"json"
	"kellegous"
	"os"
	"strconv"
	"svn"
)

const (
	webkitSvnUrl           = "http://svn.webkit.org/repository/webkit/trunk"
	webkitEarliestRevision = 48167
	modelStoreFile         = "data/model.json"
)

// Model:
// Last N WebKit commits.
// mapping from username => commits.
// last seen commit.

type model struct {
	RecentCommits     []*svn.SvnLogItem
	KittenCommits     map[string][]int64
	LastKnownRevision int64
}

func loadModelFromFile(filename string) *model {
	file, err := os.Open(modelStoreFile)
	if err != nil {
		return nil
	}

	model := &model{}
	err = json.NewDecoder(file).Decode(&model)
	if err != nil {
		return nil
	}

	return model
}

func loadModel(n int64) (*model, error) {
	// try to load the model from the storage file
	if fromFile := loadModelFromFile(modelStoreFile); fromFile != nil {
		return fromFile, nil
	}

	// Build up the model data from svn and
	// return the model when it's in a good
	// state.
	rev := webkitEarliestRevision
	count := 99594 - rev
	fmt.Printf("fetching %d revisions over %d queries.\n", count, int64(count)/n)
	client := &svn.SvnClient{webkitSvnUrl}
	items, err := client.Log(svn.REV_HEAD, svn.REV_HEAD, n)
	if err != nil {
		return nil, err
	}

	for _, item := range items {
		fmt.Printf("Revision: %d\n", item.Revision)
	}

	return nil, nil
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
	// fmt.Printf("sqlite: %s\n", sqlite.Version())
	loadModel(10)

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
