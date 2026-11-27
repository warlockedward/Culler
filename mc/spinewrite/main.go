// Command spine-write-driver is the B06 multi-process spine write lane.
//
// One process of this tool is one writer: the lane script forks N of them
// per level, so the contention measured is real multi-process contention on
// the single-writer spine. Every write runs the production verb functions
// in-process (the same surface the mc command routes to, with the host
// identity and the shared OpenSpine gate), so the measurement covers WAL,
// busy_timeout(5000), BEGIN IMMEDIATE, the guard plane, and the B10 chain
// append — the whole production write path except the CLI process boundary.
package main

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"strconv"
	"sync"
	"syscall"
	"time"

	"mc/substrate"
	"mc/verbs"
)

var (
	mode    = flag.String("mode", "writer", "provision | seed | writer | verify")
	spine   = flag.String("spine", "/mc/spine/spine.db", "the lane scratch spine file")
	levelF  = flag.String("level", "8", "the concurrency level this writer process belongs to")
	trials  = flag.Int("trials", 50, "writes performed by this process (mode=writer)")
	csvDir  = flag.String("csv", "/mc/lane/csv", "directory for per-process CSV shards")
	tokenF  = flag.String("token", "/mc/lane/token", "session token file (seed writes, writer reads)")
	userF   = flag.String("user", "/mc/lane/user.json", "seeded user receipt file (mode=seed)")
	baseF   = flag.String("seqbase", "/mc/lane/seqbase", "shared subject-serial counter file")
	handles = flag.Int("handles", 0, "independent spine handles this process holds (0 = the level)")
)

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "spine-write-driver: "+format+"\n", args...)
	os.Exit(2)
}

func randomHex(nBytes int) string {
	b := make([]byte, nBytes)
	if _, err := rand.Read(b); err != nil {
		fatal("random: %v", err)
	}
	return hex.EncodeToString(b)
}

type signoffReceipt struct {
	SignoffID string `json:"signoff_id"`
	Recorded  bool   `json:"recorded"`
}

func provision() {
	res, err := verbs.OnboardSpine(*spine, verbs.OnboardSpineRequest{
		ProtocolVersion: 1,
		SchemaID:        substrate.CurrentSchemaID,
		MirrorState:     "absent",
		Reset:           true,
	})
	if err != nil {
		fatal("onboard: %v", err)
	}
	fmt.Printf("provision status=%s deployment=%s schema=%s\n", res.Status, res.DeploymentUUID, res.SchemaID)
}

func seed() {
	db, err := verbs.OpenSpine(*spine)
	if err != nil {
		fatal("open spine: %v", err)
	}
	defer db.Close()
	user := "lane-" + randomHex(3)
	password := randomHex(16)
	if _, err := verbs.IdentityUserCreate(db, nil, verbs.IdentityUserCreateRequest{
		Username: user, Password: password,
	}); err != nil {
		fatal("user-create: %v", err)
	}
	out, err := verbs.IdentitySessionCreate(db, nil, verbs.IdentitySessionCreateRequest{
		Username: user, Password: password,
	})
	if err != nil {
		fatal("session-create: %v", err)
	}
	body, _ := json.Marshal(out)
	receipt := map[string]any{}
	if err := json.Unmarshal(body, &receipt); err != nil {
		fatal("session receipt: %v", err)
	}
	token, _ := receipt["token"].(string)
	if token == "" {
		fatal("session-create returned no token")
	}
	if err := os.WriteFile(*tokenF, []byte(token), 0o600); err != nil {
		fatal("write token file: %v", err)
	}
	if err := os.WriteFile(*userF, append(body, '\n'), 0o600); err != nil {
		fatal("write user file: %v", err)
	}
	fmt.Printf("seeded user=%s session=ok\n", user)
}

func writer() {
	startWall := time.Now().UnixNano()
	raw, err := os.ReadFile(*tokenF)
	if err != nil {
		fatal("read token: %v", err)
	}
	token := string(raw)
	level, _ := strconv.Atoi(*levelF)
	if level < 1 {
		fatal("bad level")
	}
	// One spine handle per concurrent writer: each handle carries its own
	// connection pool, BEGIN IMMEDIATE, and busy_timeout wait. File-level
	// write-lock contention depends on independent handles, not on the
	// process boundary; the lane script additionally sweeps each level as
	// distinct OS processes, so both the file-level and the literal
	// multi-process forms are measured (lane methodology declares this).
	fanOut := *handles
	if fanOut <= 0 {
		fanOut = level
	}
	dbs := make([]*sql.DB, fanOut)
	for i := range dbs {
		db, openErr := verbs.OpenSpine(*spine)
		if openErr != nil {
			fatal("open spine: %v", openErr)
		}
		dbs[i] = db
	}
	defer func() {
		for _, db := range dbs {
			db.Close()
		}
	}()
	base := reserveSerials(*baseF, fanOut**trials)
	// Container PID namespaces give every writer process the same pid, so
	// the shard name carries level, pid, start instant, and a random
	// suffix: unique across the parallel containers by construction.
	shard := fmt.Sprintf("%s/writer-%d-%d-%d-%s.csv",
		*csvDir, level, os.Getpid(), startWall, randomHex(4))
	f, err := os.Create(shard)
	if err != nil {
		fatal("create csv shard: %v", err)
	}
	defer f.Close()
	fmt.Fprintln(f, "level,pid,writer,trial,start_ns,end_ns,dur_ns,exit,recorded")
	var mu sync.Mutex
	var wg sync.WaitGroup
	for w := 0; w < fanOut; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			var lines []string
			for t := 0; t < *trials; t++ {
				subject := "b06-" + strconv.Itoa(base+w**trials+t)
				start := time.Now().UnixNano()
				code, recorded := 0, false
				res, callErr := verbs.SignoffRecord(dbs[w], nil, verbs.SignoffRecordRequest{
					Token: token, SubjectClass: "generic", SubjectRef: subject,
					Decision: "approved", Reason: "b06 envelope trial",
				})
				end := time.Now().UnixNano()
				switch {
				case callErr != nil:
					var usage *verbs.UsageError
					code = 1
					if errors.As(callErr, &usage) {
						code = 2
					}
				default:
					body, _ := json.Marshal(res)
					var r signoffReceipt
					if json.Unmarshal(body, &r) == nil {
						recorded = r.Recorded
					}
				}
				lines = append(lines, fmt.Sprintf("%d,%d,%d,%d,%d,%d,%d,%d,%d\n",
					level, os.Getpid(), w, t, start, end, end-start, code, b2i(recorded)))
			}
			mu.Lock()
			for _, line := range lines {
				fmt.Fprint(f, line)
			}
			mu.Unlock()
		}(w)
	}
	wg.Wait()
}

func verify() {
	db, err := substrate.OpenReadOnly(*spine)
	if err != nil {
		fatal("open read-only spine: %v", err)
	}
	defer db.Close()
	v, err := verbs.VerifyChain(db, "activity")
	if err != nil {
		fatal("verify-chain: %v", err)
	}
	body, _ := json.MarshalIndent(v, "", "  ")
	fmt.Println(string(body))
	m := map[string]any{}
	if json.Unmarshal(body, &m) == nil {
		if s, _ := m["status"].(string); s != "ok" {
			fmt.Fprintln(os.Stderr, "S2 STOP: the activity chain does not verify")
			os.Exit(1)
		}
	}
}

// reserveSerials takes a private serial block from a shared counter file
// under its own flock: writer processes never collide on a subject_ref.
func reserveSerials(path string, count int) int {
	h, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		fatal("open serial counter: %v", err)
	}
	defer h.Close()
	if err := syscall.Flock(int(h.Fd()), syscall.LOCK_EX); err != nil {
		fatal("lock serial counter: %v", err)
	}
	defer func() { _ = syscall.Flock(int(h.Fd()), syscall.LOCK_UN) }()
	buf := make([]byte, 32)
	n, _ := h.ReadAt(buf, 0)
	base := 0
	if n > 0 {
		if v, convErr := strconv.Atoi(string(trimSpace(buf[:n]))); convErr == nil {
			base = v
		}
	}
	next := strconv.Itoa(base + count)
	if _, err := h.WriteAt([]byte(next), 0); err != nil {
		fatal("advance serial counter: %v", err)
	}
	if err := h.Truncate(int64(len(next))); err != nil {
		fatal("truncate serial counter: %v", err)
	}
	return base
}

func trimSpace(b []byte) []byte {
	for len(b) > 0 && (b[len(b)-1] == '\n' || b[len(b)-1] == ' ') {
		b = b[:len(b)-1]
	}
	return b
}

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}

func main() {
	flag.Parse()
	switch *mode {
	case "provision":
		provision()
	case "seed":
		seed()
	case "writer":
		writer()
	case "verify":
		verify()
	default:
		fatal("unknown mode %q", *mode)
	}
}
