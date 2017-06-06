package comparisonScheduler

import (
    "database/sql"
    "errors"
    "log"
    "math"
    "math/rand"
    "time"
    _ "github.com/lib/pq"
    "github.com/chrismamo1/reflagvsflag/things"
    "github.com/chrismamo1/reflagvsflag/users")

type Priority int

const (
    PMarginal Priority = iota
    PLow
    PMedium
    PHigh
)

type Scheduler struct {
    db *sql.DB
    pointlessThreshold int
}

func (this *Scheduler) hasAnyRequests() bool {
    var status bool
    query := `SELECT EXISTS(SELECT * FROM schedule LIMIT 1)`
    if err := this.db.QueryRow(query).Scan(&status); err != nil {
        log.Fatal("Error while trying to see if the scheduler table has anything in it: ", err)
    }
    return status
}

func (this *Scheduler) getMinPlacement() int {
    placement := 1
    if this.hasAnyRequests() {
        query := `select MIN(placement) FROM scheduler`
        if err := this.db.QueryRow(query).Scan(&placement); err != nil {
            log.Fatal("Error while getting the max placement from the scheduler while appending a request: ", err)
        }
    }
    return placement
}

func (this *Scheduler) hasRequest(ids things.IDPair) bool {
    var status bool
    query := `SELECT EXISTS(SELECT * FROM schedule WHERE (fst = $1 AND snd = $2) OR (fst = $2 AND snd = $1) LIMIT 1)`
    if err := this.db.QueryRow(query, ids.Fst, ids.Snd).Scan(&status); err != nil {
        log.Fatal("Error while trying to determine if the scheduler contains a request: ", err)
    }
    return status
}

func (this *Scheduler) rmRequest(ids things.IDPair) {
    statement := `DELETE FROM schedule WHERE (fst = $1 AND snd = $2) OR (fst = $2 AND snd = $1)`
    if _, err := this.db.Exec(statement, ids.Fst, ids.Snd); err != nil {
        log.Fatal("Error while trying to delete a scheduler request: ", err)
    }
}

func (this *Scheduler) HasRequest(ids things.IDPair) bool {
    return this.hasRequest(ids)
}

func (this *Scheduler) FillRequest(winner things.ID, loser things.ID) {
    ids := things.IDPair{Fst: winner, Snd: loser}
    log.Printf("Filling a request for %d, %d\n", int(winner), int(loser))
    if this.hasRequest(ids) {
        var elo1, elo2 float64
        query := `SELECT elo FROM images WHERE id = $1;`
        if err := this.db.QueryRow(query, winner).Scan(&elo1); err != nil {
            log.Fatal("Error getting 1st elo in FillRequest: ", err)
        }
        if err := this.db.QueryRow(query, loser).Scan(&elo2); err != nil {
            log.Fatal("Error getting 2nd elo in FillRequest: ", err)
        }
        r1 := math.Pow(10.0, elo1 / 400)
        r2 := math.Pow(10.0, elo2 / 400)
        e1 := r1 / (r1 + r2)
        e2 := r2 / (r1 + r2)
        s1 := 1.0
        s2 := 0.0
        elo1 = elo1 + 10.0 * (s1 - e1)
        elo2 = elo2 + 10.0 * (s2 - e2)
        log.Printf("New ELO for image %d: %f\n", int(winner), elo1)
        log.Printf("New ELO for image %d: %f\n", int(loser), elo2)
        statement := "UPDATE images SET elo = $1 WHERE id = $2"
        if _, err := this.db.Exec(statement, elo1, winner); err != nil {
            log.Fatal("Error while trying to update an ELO value: ", err)
        }
        statement = "UPDATE images SET elo = $1 WHERE id = $2"
        if _, err := this.db.Exec(statement, elo2, loser); err != nil {
            log.Fatal("Error while trying to update an ELO value: ", err)
        }
        this.rmRequest(ids)
        if this.hasRequest(ids) {
            log.Fatal(errors.New("rmRequest doesn't work"))
        }
    }
}

func (this *Scheduler) NextRequest(user users.User, tags []string) things.IDPair {
    tx := things.GetTransactionWithTags(this.db, tags)

    rand.Seed(time.Now().UnixNano())

    var ids things.IDPair
    var s_heat int
    var elo float64

    query := `
        SELECT id, COALESCE(views.heat, 0) + COALESCE(imgs.heat, 0) AS s_heat, COALESCE(elo)
        FROM
            views,
            imgs
        WHERE "user" = $1
        GROUP BY (id, s_heat)
        ORDER BY s_heat ASC LIMIT 1
    `
    if err := tx.QueryRow(query, user.Id).Scan(&ids.Fst, &s_heat, &elo); err != nil {
        log.Println(err)
        if err := tx.Rollback(); err != nil {
            log.Fatal("Error while trying to rollback the aborted transaction: ", err)
        }

        tx := things.GetTransactionWithTags(this.db, tags)
        query := `
            SELECT imgs.id
            FROM imgs
            ORDER BY imgs.heat ASC, RANDOM()
            LIMIT 2
        `
        rows, err := tx.Query(query)
        if err != nil {
            log.Fatal("Error selecting totally random elements in NextRequest: ", err)
        }
        rows.Next()
        if err := rows.Scan(&ids.Fst); err != nil {
            log.Fatal("Error while scanning the first ID in NextRequest: ", err)
        }
        rows.Next()
        if err := rows.Scan(&ids.Snd); err != nil {
            log.Fatal("Error while scanning the second ID in NextRequest: ", err)
        }
        rows.Close()
    } else {
        query := `
            SELECT id
            FROM
                (SELECT *
                 FROM imgs
                 ORDER BY ABS(elo-$1) ASC
                 LIMIT 10) tbl
            ORDER BY RANDOM() LIMIT 1;
        `
        if err := tx.QueryRow(query, elo).Scan(&ids.Snd); err != nil {
            log.Fatal("Error selecting: ", err)
        }
    }

    statement := `INSERT INTO schedule (fst, snd, "user") VALUES ($1, $2, $3)`
    if _, err := tx.Exec(statement, ids.Fst, ids.Snd, user.Id); err != nil {
        log.Fatal("Error trying to add an element to the schedule: ", err)
    }

    tx.Commit()
    return ids;
}

func Make(db *sql.DB, pointlessAt int) *Scheduler {
    return &Scheduler{db: db, pointlessThreshold: pointlessAt}
}

/*func (this *Scheduler) addSatisfaction(ids things.IDPair) {
    n := &node{x: ids, next: this.satisfactions}
    this.requests = n
}

func (this *Scheduler) AddSatisfaction(ids things.IDPair) {
    this.mux.Lock()
    defer this.mux.Unlock()

    this.addSatisfaction(ids)
}

func (this *Scheduler) rmSatisfaction(ids things.IDPair) {
    for n := this.satisfactions; n != nil; n = n.next {
        if n.x.Equivalent(ids) {
            *n = n->next
        }
    }
}

func (this *Scheduler) RmSatisfaction(ids things.IDPair) {
    this.mux.Lock()
    defer this.mux.Unlock()

    this.rmSatisfaction(ids)
}

func (this *Scheduler) HasSatisfaction(ids things.IDPair) bool {
    this.mux.Lock()
    defer this.mux.Unlock()

    for n := this.satisfactions; n != nil; n = n.next {
        if n.x.Equivalent(ids) {
            return true
        }
    }
    return false
}

func (this *Scheduler) NextRequest() {
    if this.requests == nil {
        if this.satisfactions != nil {
            this.addRequest(this.satisfactions.x)
        }
    } else {
        ids := this.requests.x
        this.rmRequest(ids)
        this.addSatisfaction(ids)
    }
}*/

/*type sorter struct {
    reqs chan things.IDPair
    last things.IDPair
    '
    resps
}

type Scheduler struct {
    sorters []sorter
    controllerReqs chan things.IDPair
    controllerResps chan things.IDPair
    db *sql.DB
}

func (this *Scheduler) Run() {
    requests := make([]things.IDPair, 512)
    for {
        select {
        case r := <-this.controllerResps:
            for i, v := range requests {
                if r.Equivalent(v) {
                    sorterResps <- r
                    for j := i + 1; j < cap(requests); j = j + 1 {
                        requests[j - 1] = requests[j]
                    }
                    requests := requests[:(cap(requests) - 1)]
                }
            }
        }
        for i, s := range sorters {
            select {
            case r := <-s.reqs:
                s.last := r
            }
        }
    }
}

func (this *Scheduler) AddSorter(reqs chan things.IDPair, resps chan things.IDPair) {
    var s sorter
    s.reqs = reqs
    append(this.sorters, s)
}

 //@param reqs the channel used by flagSort to send comparison requests
 //@param resps the channel used by flagSort to wait for comparison responses
 //@param db the database handle
 //
func Make(reqs chan things.IDPair, resps chan things.IDPair, db *sql.DB) Scheduler {
    var rval Scheduler

    rval.sorters = make([]sorter)
    rval.AddSorter(reqs, resps)

    // it's OK for the controller to be non-blocking (I think)
    rval.controllerReqs = make(chan things.IDPair, 256)
    rval.controllerResps = resps

    rval.db = db

    return rval
}

func (this *Scheduler) GetControllerChannels() chan things.IDPair, chan things.IDPair {
    return this.controllerReqs, this.ControllerResps
}*/
