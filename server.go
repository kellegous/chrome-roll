package main

import (
  "crypto/sha1"
	"encoding/json"
  "flag"
	"fmt"
  "gosqlite.googlecode.com/hg/sqlite"
  "io"
	"kellegous"
  "log"
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
  webkitSvnPollingInterval = 1 // minutes
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
  q := "SELECT Revision, Comment, Date, Author FROM change ORDER BY Revision DESC"
  if limit >= 0 {
    q += fmt.Sprintf(" LIMIT %d", limit)
  }
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

func (s *store) insertChange(revision int64, comment, date, author string) error {
  q := "INSERT OR REPLACE INTO change VALUES (?1, ?2, ?3, ?4)"
  if err := s.Conn.Exec(q, revision, comment, date, author); err != nil {
    return err
  }
  return nil
}

func (s *store) insertKittenChange(email string, revision int64) error {
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
var patternForOldLogs = regexp.MustCompile("^\\d{4}-\\d{2}-\\d{2}  .*  <(.*)>")

func isKittenChange(kitten *kitten, author, comment string) bool {
  if author == kitten.Email {
    return true
  }

  for _, m := range patternForPatchBy.FindAllStringSubmatch(comment, -1) {
    if m[1] == kitten.Email {
      return true
    }
  }

  for _, m := range patternForOldLogs.FindAllStringSubmatch(comment, -1) {
    if m[1] == kitten.Email {
      return true
    }
  }

  return false
}

type changeMessage struct {
  Type string
  Change *change
  Kittens []string
}

func newChangeMessage(change *change, kittens []string) *changeMessage {
  return &changeMessage{"change", change, kittens};
}

type connectMessage struct {
  Type string
  Changes []*change
  Kittens []*kitten
  Version string
}

func newConnectMessage(changes []*change, kittens []*kitten, versionIdentifier string) *connectMessage {
  return &connectMessage{"connect", changes, kittens, versionIdentifier};
}

type model struct {
  Kittens []*kitten
  Changes []*change
  Store *store
  Svn *svn.Client
  Conns map[*websocket.Conn]int
  VersionIdentifier string
}

func loadModel(sqlFilename, svnUrl, versionIdentifier string) (*model, error) {
  m := &model{}

  m.Svn = &svn.Client{svnUrl}
  m.VersionIdentifier = versionIdentifier

  store, err := newStore(sqlFilename)
  if err != nil {
    return nil, err
  }
  m.Store = store

  m.Conns = map[*websocket.Conn]int{}

  if err := m.reload(); err != nil {
    return nil, err
  }

  return m, nil
}

func (m *model) reload() error {
  kittens, err := m.Store.kittens()
  if err != nil {
    return err
  }
  m.Kittens = kittens

  changes, err := m.Store.changes(100)
  if err != nil {
    return err
  }
  m.Changes = changes
  return nil
}

func (m *model) update() error {
  latestRev := webkitEarliestRevision
  if len(m.Changes) != 0 {
    latestRev = m.Changes[0].Revision
  }

  items, err := m.Svn.Log(latestRev, svn.REV_HEAD, svn.LIMIT_NONE)
  if err != nil {
    return err
  }

  for _, item := range items {
    if item.Revision == latestRev {
      continue
    }

    log.Printf("  update with r%d by %s\n", item.Revision, item.Author)
    n := newChangeMessage(&change{item.Revision, item.Comment, item.Date, item.Author}, []string{})
    err := m.Store.insertChange(item.Revision, item.Comment, item.Date, item.Author)
    if err != nil {
      return err
    }

    for _, kitten := range m.Kittens {
      if !isKittenChange(kitten, item.Author, item.Comment) {
        continue
      }

      n.Kittens = append(n.Kittens, kitten.Email)
      err := m.Store.insertKittenChange(kitten.Email, item.Revision)
      if err != nil {
        return err
      }
    }

    m.notify(n)
  }

  if err := m.reload(); err != nil {
    return err
  }

  return nil
}

func (m *model) rebuildKittenChangeTable() error {
  log.Printf("rebuilding kitten change table\n")
  changes, err := m.Store.changes(-1)
  if err != nil {
    return err
  }

  for _, change := range changes {
    for _, kitten := range m.Kittens {
      if !isKittenChange(kitten, change.Author, change.Comment) {
        continue
      }

      log.Printf("  %s made r%d\n", kitten.Email, change.Revision)
      err = m.Store.insertKittenChange(kitten.Email, change.Revision)
      if err != nil {
        return err
      }
    }
  }

  if err := m.reload(); err != nil {
    return err
  }

  return nil
}

func (m *model) unsubscribe(s *websocket.Conn) {
  s.Close()
  m.Conns[s] = 0, false
}

func (m *model) subscribe(s *websocket.Conn) {
  m.Conns[s] = 1
  m.notify(newConnectMessage(m.Changes, m.Kittens, m.VersionIdentifier))
}

func (m *model) notify(n interface{}) error {
  data, err := json.MarshalIndent(n, "", "  ")
  if err != nil {
    return err
  }

  for c, _ := range m.Conns {
    c.Write(data)
  }

  return nil
}

func startModel(ch chan *sub, svnUrl, storeFile, versionIdentifier string, rebuildChangeTable bool) error {
  log.Printf("loading model (%s, %s, %s)\n", storeFile, svnUrl, versionIdentifier)
  model, err := loadModel(storeFile, svnUrl, versionIdentifier)
  if err != nil {
    return err
  }

  log.Printf("updating model to HEAD\n")
  if err := model.update(); err != nil {
    return err
  }

  if rebuildChangeTable {
    if err := model.rebuildKittenChangeTable(); err != nil {
      return err
    }
  }

  go func() {
    log.Printf("log started\n")
    for {
      select {
      case s :=  <-ch:
        if s.isOpen {
          log.Printf("client accepted from %s\n", s.socket.RemoteAddr().String())
          model.subscribe(s.socket)
        } else {
          log.Printf("client closed from %s\n", s.socket.RemoteAddr().String())
          model.unsubscribe(s.socket)
        }
      case <- time.After(webkitSvnPollingInterval * 60 * 1e9):
        log.Printf("updating model to HEAD\n")
        model.update()
        // TODO: handle errors here ... simply log them.
      }
    }
  }()
  return nil
}

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

type sub struct {
  socket *websocket.Conn
  isOpen bool
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

var flagAddr = flag.String("addr",
    ":6565",
    "bind address for http server")

var flagRebuildChangeTable = flag.Bool("rebuild-change-table",
    false,
    "")

func readVersionIdentifier() (string, error) {
  i, err := os.Open(os.Args[0])
  if err != nil {
    return "", err
  }

  s := sha1.New()
  _, err = io.Copy(s, i)
  if err != nil {
    return "", err
  }

  return fmt.Sprintf("%x", s.Sum()), nil
}

func main() {
  flag.Parse()

  version, err := readVersionIdentifier()
  if err != nil {
    panic(err)
  }

  // channel allows websockets to attach to model.
  wsChan := make(chan *sub)
  err = startModel(wsChan, webkitSvnUrl, modelDatabaseFile, version, *flagRebuildChangeTable)
  if err != nil {
    panic(err)
  }

	http.Handle("/chrome/", newSvnLogHandler("http://src.chromium.org/svn/trunk"))
	http.Handle("/", kellegous.NewAppHandler(http.Dir("pub")))
	http.Handle("/atl/str", websocket.Handler(func(ws *websocket.Conn) {
    wsChan <- &sub{ws, true}

    for {
      b := make([]byte, 1)
      _, err := ws.Read(b)
      if err != nil {
        wsChan <- &sub{ws, false}
        ws.Close()
        return
      }
    }
  }))

	err = http.ListenAndServe(*flagAddr, nil)
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
  `INSERT OR REPLACE INTO kitten VALUES('schenney@chromium.org', 'Stephen Chenney');`,
  `INSERT OR REPLACE INTO kitten VALUES('pdr@google.com', 'Philip Rogers');`,
  `INSERT OR REPLACE INTO kitten VALUES('fmalita@google.com', 'Florin Malita');`}
