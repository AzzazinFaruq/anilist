// Command tonton runs the family anime tracker and manages its accounts.
//
//	tonton serve                       start the web server
//	tonton user add <name> [--admin]   add a member (PIN read from stdin or TONTON_PIN)
//	tonton user pin <name>             set a new PIN
//	tonton user rename <name> <new>
//	tonton user list
//	tonton import <name> <state.json>  copy a list from the old Arsip sync file
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"
	_ "time/tzdata" // the scratch image has no zoneinfo

	"tonton/internal/api"
	"tonton/internal/auth"
	"tonton/internal/catalog"
	"tonton/internal/store"
	"tonton/internal/web"
)

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lmsgprefix)
	log.SetPrefix("tonton: ")
	args := os.Args[1:]
	if len(args) == 0 {
		args = []string{"serve"}
	}
	var err error
	switch args[0] {
	case "serve":
		err = serve()
	case "user":
		err = userCmd(args[1:])
	case "import":
		err = importCmd(args[1:])
	case "healthcheck":
		err = healthcheck()
	default:
		err = fmt.Errorf("unknown command %q (serve, user, import)", args[0])
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "tonton:", err)
		os.Exit(1)
	}
}

func openStore() (*store.Store, error) { return store.Open(env("TONTON_DATA", "/data")) }

func serve() error {
	loc, err := time.LoadLocation(env("TZ", "Asia/Jakarta"))
	if err != nil {
		return err
	}
	cat, err := catalog.Open(env("TONTON_CATALOG", "/catalog"), os.Getenv("TONTON_SYNOPSIS"))
	if err != nil {
		return err
	}
	st, err := openStore()
	if err != nil {
		return err
	}
	key, err := st.Key()
	if err != nil {
		return err
	}
	srv := &api.Server{
		Cat: cat, Store: st, Loc: loc, Static: web.Files(),
		Sessions: auth.Sessions{Key: key, TTL: 180 * 24 * time.Hour, Now: time.Now},
		Limiter:  &auth.Limiter{Free: 5, Max: 15 * time.Minute, Now: time.Now},
		AnyIP:    os.Getenv("TONTON_ALLOW_ANY_IP") == "1",
	}
	if srv.Static == nil {
		log.Print("built web app missing: serving the API only")
	}
	stop := make(chan struct{})
	go cat.Watch(5*time.Minute, stop, log.Printf)

	addr := env("TONTON_ADDR", ":8080")
	hs := &http.Server{
		Addr: addr, Handler: srv.Handler(),
		ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second,
		WriteTimeout: 30 * time.Second, IdleTimeout: 2 * time.Minute,
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	go func() {
		<-ctx.Done()
		close(stop)
		shut, done := context.WithTimeout(context.Background(), 10*time.Second)
		defer done()
		_ = hs.Shutdown(shut)
	}()
	log.Printf("listening on %s with %d titles and %d members", addr, cat.Len(), len(st.Users()))
	if err := hs.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// healthcheck lets the container check itself without curl in the image.
func healthcheck() error {
	addr := env("TONTON_ADDR", ":8080")
	if strings.HasPrefix(addr, ":") {
		addr = "127.0.0.1" + addr
	}
	c := http.Client{Timeout: 3 * time.Second}
	res, err := c.Get("http://" + addr + "/healthz")
	if err != nil {
		return err
	}
	res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("status %d", res.StatusCode)
	}
	return nil
}

func readPIN() (string, error) {
	if p := os.Getenv("TONTON_PIN"); p != "" {
		return p, nil
	}
	fmt.Fprint(os.Stderr, "PIN: ")
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil && line == "" {
		return "", errors.New("no PIN given (pipe it on stdin or set TONTON_PIN)")
	}
	return strings.TrimSpace(line), nil
}

func userCmd(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: tonton user add|pin|rename|list")
	}
	st, err := openStore()
	if err != nil {
		return err
	}
	switch args[0] {
	case "list":
		for _, u := range st.Users() {
			role := "anggota"
			if u.Admin {
				role = "admin"
			}
			fmt.Printf("%s\t%s\t%s\n", u.ID, u.Name, role)
		}
		return nil
	case "add":
		fs := flag.NewFlagSet("user add", flag.ContinueOnError)
		admin := fs.Bool("admin", false, "can manage family members")
		if err := fs.Parse(reorder(args[1:])); err != nil {
			return err
		}
		if fs.NArg() != 1 {
			return errors.New("usage: tonton user add <name> [--admin]")
		}
		pin, err := readPIN()
		if err != nil {
			return err
		}
		hash, err := auth.HashPIN(pin)
		if err != nil {
			return err
		}
		u, err := st.AddUser(fs.Arg(0), hash, *admin)
		if err != nil {
			return err
		}
		fmt.Printf("added %s (%s)\n", u.Name, u.ID)
		return nil
	case "rename":
		if len(args) != 3 {
			return errors.New("usage: tonton user rename <name> <new name>")
		}
		u, ok := st.UserByName(args[1])
		if !ok {
			return fmt.Errorf("no member named %q", args[1])
		}
		_, err := st.Rename(u.ID, args[2])
		return err
	case "pin":
		if len(args) != 2 {
			return errors.New("usage: tonton user pin <name>")
		}
		u, ok := st.UserByName(args[1])
		if !ok {
			return fmt.Errorf("no member named %q", args[1])
		}
		pin, err := readPIN()
		if err != nil {
			return err
		}
		hash, err := auth.HashPIN(pin)
		if err != nil {
			return err
		}
		_, err = st.SetHash(u.ID, hash)
		return err
	}
	return fmt.Errorf("unknown user command %q", args[0])
}

// reorder moves flags in front of positional arguments so "add rina --admin" works.
func reorder(args []string) []string {
	var flags, rest []string
	for _, a := range args {
		if strings.HasPrefix(a, "-") {
			flags = append(flags, a)
		} else {
			rest = append(rest, a)
		}
	}
	return append(flags, rest...)
}

// importCmd copies entries from the old Arsip sync file (state.json, field "my").
func importCmd(args []string) error {
	if len(args) != 2 {
		return errors.New("usage: tonton import <name> <state.json>")
	}
	st, err := openStore()
	if err != nil {
		return err
	}
	u, ok := st.UserByName(args[0])
	if !ok {
		return fmt.Errorf("no member named %q", args[0])
	}
	b, err := os.ReadFile(args[1])
	if err != nil {
		return err
	}
	var old struct {
		My map[string]struct {
			At  int64  `json:"at"`
			Del int    `json:"del"`
			S   string `json:"s"`
			Ep  int    `json:"ep"`
			R   int    `json:"r"`
			N   string `json:"n"`
			Sa  int64  `json:"sa"`
			Fa  int64  `json:"fa"`
		} `json:"my"`
	}
	if err := json.Unmarshal(b, &old); err != nil {
		return err
	}
	entries := map[int]store.Entry{}
	for k, v := range old.My {
		id, err := strconv.Atoi(k)
		if err != nil || v.Del != 0 || !store.ValidStatus(v.S) {
			continue // special keys (_log, _sel …) and deleted entries
		}
		updated := v.At
		if updated < 1e12 { // the old migration stamped entries with at=1
			updated = time.Now().UnixMilli()
		}
		note := v.N
		if r := []rune(note); len(r) > 500 {
			note = string(r[:500])
		}
		entries[id] = store.Entry{
			Status: v.S, Episode: max(0, v.Ep), Rating: min(max(0, v.R), 10), Note: note,
			Updated: updated, Started: v.Sa, Finished: v.Fa,
		}
	}
	n, err := st.Import(u.ID, entries)
	if err != nil {
		return err
	}
	fmt.Printf("imported %d of %d entries into %s\n", n, len(entries), u.Name)
	return nil
}
