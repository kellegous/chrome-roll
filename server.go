package main

import (
	"encoding/json"
  "flag"
	"fmt"
  "gosqlite.googlecode.com/hg/sqlite"
	"kellegous"
	"net/http"
  "os"
  "path/filepath"
  "regexp"
	"strconv"
	"svn"
  "time"
  "websocket"
)

const (
	webkitSvnUrl           = "http://svn.webkit.org/repository/webkit/trunk"
	webkitEarliestRevision int64 = 48167
	modelDatabaseFile      = "db/webkit.sqlite"

)

type kitten struct {
  Email string
  Name string
  Revisions []int64
}
func (k *kitten) add(revision int64) {
  k.Revisions = append(k.Revisions, revision)
}

type store struct {
  Conn *sqlite.Conn
}

func newStore(filename string) (*store, error) {
  // make sure the parent directory exists
  dir, _ := filepath.Split(filename)
  if s, _ := os.Stat(dir); s == nil {
    os.MkdirAll(dir, 0700)
  }

  // open the database
  c, err := sqlite.Open(filename)
  if err != nil {
    return nil, err
  }

  // execute all commands in the setup script
  for _, s := range databaseSetupScript {
    if err := c.Exec(s); err != nil {
      return nil, err
    }
  }

  return &store{c}, nil
}

func (s *store) kittens() ([]*kitten, error) {
  kits := make([]*kitten, 0)
  q := "SELECT a.Email, a.Name, b.Revision FROM kitten a LEFT OUTER JOIN kittenchange b ON a.Email = b.Email ORDER BY a.Email, b.Revision"
  stmt, err := s.Conn.Prepare(q)
  if err != nil {
    return nil, err
  }

  if err := stmt.Exec(); err != nil {
    return nil, err
  }

  var kit *kitten = nil
  var email, name, revision string

  for i := 0; stmt.Next(); i++ {
    if err := stmt.Scan(&email, &name, &revision); err != nil {
      return nil, err
    }

    if kit == nil || kit.Email != email {
      kit = &kitten{email, name, []int64{}}
      kits = append(kits, kit)
    }

    if revision != "" {
	    r, err := strconv.Atoi64(revision)
      if err != nil {
        return nil, err
      }
      kit.add(r)
    }
  }

  return kits, nil
}

func (s *store) changes(limit int) ([]*change, error) {
  changes := []*change{}
  q := fmt.Sprintf("SELECT Revision, Comment, Date, Author FROM change ORDER BY Revision DESC LIMIT %d", limit)
  stmt, err := s.Conn.Prepare(q)
  if err != nil {
    return nil, err
  }

  if err := stmt.Exec(); err != nil {
    return nil, err
  }

  var revision int64
  var comment, date, author string

  for ; stmt.Next(); {
    if err := stmt.Scan(&revision, &comment, &date, &author); err != nil {
      return nil, err
    }

    changes = append(changes, &change{revision, comment, date, author})
  }

  return changes, nil
}

func (s *store) insertCommit(revision int64, comment, date, author string) error {
  q := "INSERT OR REPLACE INTO change VALUES (?1, ?2, ?3, ?4)"
  if err := s.Conn.Exec(q, revision, comment, date, author); err != nil {
    return err
  }
  return nil
}

func (s *store) insertKittenCommit(email string, revision int64) error {
  q := "INSERT OR REPLACE INTO kittenchange VALUES (?1, ?2)"
  if err := s.Conn.Exec(q, email, revision); err != nil {
    return err
  }
  return nil
}

type change struct {
  Revision int64
  Comment string
  Date string
  Author string
}

func (s *store) shutdown() {
  s.Conn.Close()
}

var patternForPatchBy = regexp.MustCompile("Patch by .* <(.*)> on")
var patternForOldLogs = regexp.MustCompile("\n\\d{4}-\\d{2}\\d{2}  .*  <(.*)>")

func isKittenChange(c *change, email string) bool {
  if c.Author == email {
    return true
  }

  for _, m := range patternForPatchBy.FindAllStringSubmatch(c.Comment, -1) {
    if m[1] == email {
      return true
    }
  }

  for _, m := range patternForOldLogs.FindAllStringSubmatch(c.Comment, -1) {
    if m[1] == email {
      return true
    }
  }

  return false
  // Look for Date - name - email format
}

func update(st *store, sc *svn.Client, start int64) (int64, bool, error) {
  head, err := sc.Head()
  if err != nil {
    return 0, false, nil
  }

  items, err := sc.Log(start, head.Revision, 100)
  if err != nil {
    return 0, false, nil
  }

  var lastRev int64 = 0
  for _, i := range items {
    fmt.Println(i.Revision)
    lastRev = i.Revision
    err = st.insertCommit(i.Revision, i.Comment, i.Date, i.Author)
    if err != nil {
      return lastRev, false, err
    }
  }

  return lastRev, lastRev == head.Revision, nil
}

func startModel(ch chan *websocket.Conn, svnUrl string, storeFile string) error {
  st, err := newStore(storeFile)
  if err != nil {
    return err
  }

  changes, err := st.changes(100)
  if err != nil {
    return err
  }

  head := webkitEarliestRevision
  if len(changes) > 0 {
    head = changes[0].Revision
  }

  sc := &svn.Client{svnUrl}

  go func() {
    // TODO: Setup a timeout that is initially really low.
    // we will then enter the loop with low time outs and
    // incrementally get up-to-date. When we make it to head,
    // we'll then crank up the timeout to our polling interval
    // and proceed from there.
    cons := make([]*websocket.Conn, 0)
    for {
      select {
      case c :=  <-ch:
        cons = append(cons, c)
        // Send the model state to the client.
      case <- time.After(1e9):
        // Perform incremental update.
        _, _, err := update(st, sc, head)
        if err != nil {
          fmt.Println(err)
        }
        return
      }
    }
  }()
  return nil
}

  // for last-commit to head
  //   for each kitten
  //     if kitten in commit
  //       add kittencommit to store.
  //   send notification of commit
  //   update last-commit

// An http.Handler that allows JSON access to log data.
type svnLogHandler struct {
	client *svn.Client
}

func newSvnLogHandler(url string) *svnLogHandler {
	return &svnLogHandler{&svn.Client{url}}
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
var flagBuildDatabase = flag.Bool("build-database", false, "Creates the webkit database and then exits.")

func main() {
  flag.Parse()

  // channel allows websockets to attach to model.
  wsChan := make(chan *websocket.Conn)
  err = startModel(wsChan, webkitSvnUrl, modelDatabaseFile)
  if err != nil {
    panic(err)
  }

	http.Handle("/chrome/", newSvnLogHandler("http://src.chromium.org/svn/trunk"))
	http.Handle("/", kellegous.NewAppHandler(http.Dir("pub")))
	http.Handle("/atl/str", websocket.Handler(func(ws *websocket.Conn) {
    wsChan <- ws
  }))
	fmt.Println("Running...")
	err = http.ListenAndServe(":6565", nil)
	if err != nil {
		panic(err)
	}
}

var databaseSetupScript = []string{`
  CREATE TABLE IF NOT EXISTS kitten (
    Email varchar(255) PRIMARY KEY,
    Name varchar(255)
  );`,
  `
  CREATE TABLE IF NOT EXISTS kittenchange (
    Email varchar(255),
    Revision integer,
    CONSTRAINT _key PRIMARY KEY (Email, Revision)
  );`,
 ` 
  CREATE TABLE IF NOT EXISTS change (
    Revision INTEGER PRIMARY KEY,
    Comment TEXT,
    Date VARCHAR(255),
    Author VARCHAR(255)
  );`,
  `INSERT OR REPLACE INTO kitten VALUES('knorton@google.com', 'Kelly Norton');`,
  `INSERT OR REPLACE INTO kitten VALUES('jgw@google.com', 'Joel Webber');`,
  `INSERT OR REPLACE INTO kitten VALUES('schenney@google.com', 'Stephen Chenney');`,
  `INSERT OR REPLACE INTO kitten VALUES('pdr@google.com', 'Philip Rogers');`,
  `INSERT OR REPLACE INTO kitten VALUES('fmalita@google.com', 'Florin Malita');`}
