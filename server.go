package main

import (
  "flag"
	"fmt"
  "gosqlite.googlecode.com/hg/sqlite"
	"http"
	"json"
	"kellegous"
	"strconv"
	"svn"
  "websocket"
)

const (
	webkitSvnUrl           = "http://svn.webkit.org/repository/webkit/trunk"
	webkitEarliestRevision = 48167
	modelDatabaseFile      = "db/webkit.sqlite"

)

// Model:
// Last N WebKit commits.
// mapping from username => commits.
// last seen commit.

type kitten struct {
  Email string
  Name string
  Commits []int64
}
func (k *kitten) add(commit int64) {
  k.Commits = append(k.Commits, commit)
}

type store struct {
  Conn *sqlite.Conn
}
func newStore(filename string) (*store, error) {
  c, err := sqlite.Open(filename)
  if err != nil {
    return nil, err
  }
  for _, s := range databaseSetupScript {
    if err := c.Exec(s); err != nil {
      return nil, err
    }
  }
  return &store{c}, nil
}
func (s *store) kittens() ([]*kitten, error) {
  kits := make([]*kitten, 0)
  q := "SELECT a.Email, a.Name, b.Revision FROM kitten a LEFT OUTER JOIN kittencommit b ON a.Email = b.Email ORDER BY a.Email, b.Revision"
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
      fmt.Println(err)
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
func (s *store) lastCommit() (int64, error) {
  q := "SELECT Value FROM setting WHERE Name = 'last-revision'"
  stmt, err := s.Conn.Prepare(q)
  if err != nil {
    return 0, err
  }

  if err := stmt.Exec(); err != nil {
    return 0, err
  }

  if !stmt.Next() {
    return 0, nil
  }

  var revision int64
  if err := stmt.Scan(&revision); err != nil {
    return 0, err
  }

  return revision, nil
}
func (s *store) setLastCommit(revision int64) error {
  q := "INSERT OR REPLACE INTO setting VALUES ('last-revision', ?1)"
  if err := s.Conn.Exec(q, revision); err != nil {
    return err
  }
  return nil
}
func (s *store) addKittenCommit(name string, revision int64) error {
  q := "INSERT OR REPLACE INTO kittencommit VALUES (?1, ?2)"
  if err := s.Conn.Exec(q, name, revision); err != nil {
    return err
  }
  return nil
}
func (s *store) shutdown() {
  s.Conn.Close()
}

func startModel(ch chan *websocket.Conn, svnUrl string, storeFile string) error {
  sql, err := newStore(storeFile)
  if err != nil {
    return err
  }

  rev, err := sql.lastCommit()
  if err != nil {
    return err
  }

  scm := &svn.SvnClient{svnUrl}
  _, err = scm.Log(rev, svn.REV_FIRST, 100)
  if err != nil {
    return err
  }

  go func() {
    cons := make([]*websocket.Conn, 0)
    for {
      select {
      case c :=  <-ch:
        cons = append(cons, c)
        fmt.Println("We have us a connection.")
        // Send the client the data dude.
      // TODO: Add Case for timeout here.
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
var flagBuildDatabase = flag.Bool("build-database", false, "Creates the webkit database and then exits.")

func showKittens(store *store) {
  kits, err := store.kittens()
  if err != nil {
    panic(err)
  }
  for _, k := range kits {
    fmt.Println(k)
  }
}

func main() {
  flag.Parse()

  // channel allows websockets to attach to model.
  wsChan := make(chan *websocket.Conn)
  err := startModel(wsChan, webkitSvnUrl, modelDatabaseFile)
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
  CREATE TABLE IF NOT EXISTS kittencommit (
    Email varchar(255),
    Revision integer,
    CONSTRAINT _key PRIMARY KEY (Email, Revision)
  );`,
  `
  CREATE TABLE IF NOT EXISTS setting (
    Name varchar(255) PRIMARY KEY,
    Value varchar(255)
  );`,
  `INSERT OR REPLACE INTO kitten VALUES('knorton@google.com', 'Kelly Norton');`,
  `INSERT OR REPLACE INTO kitten VALUES('jgw@google.com', 'Joel Webber');`,
  `INSERT OR REPLACE INTO kitten VALUES('schenney@google.com', 'Stephen Chenney');`,
  `INSERT OR REPLACE INTO kitten VALUES('pdr@google.com', 'Philip Rogers');`,
  `INSERT OR REPLACE INTO kitten VALUES('fmalita@google.com', 'Florin Malita');`}
