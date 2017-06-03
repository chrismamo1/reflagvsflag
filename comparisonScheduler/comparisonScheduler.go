package comparisonScheduler

import (
    "database/sql"
    "errors"
    "log"
    "runtime"
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
    query := `SELECT EXISTS(SELECT * FROM scheduler LIMIT 1)`
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

func (this *Scheduler) getMaxPlacement() int {
    placement := -1
    if this.hasAnyRequests() {
        query := `select MAX(placement) FROM scheduler`
        if err := this.db.QueryRow(query).Scan(&placement); err != nil {
            log.Fatal("Error while getting the max placement from the scheduler while appending a request: ", err)
        }
    }
    return placement
}

func (this *Scheduler) prependRequest(ids things.IDPair, p Priority) {
    placement := this.getMinPlacement() - 1
    statement := `
        INSERT INTO scheduler (fst, snd, placement, priority)
            VALUES
                (   $1,
                    $2,
                    $3,
                    $4);`
    if _, err := this.db.Exec(statement, ids.Fst, ids.Snd, placement, p); err != nil {
        log.Fatal("Error while inserting a request at the front of the queue: ", err)
    }
    return
}

func (this *Scheduler) appendRequest(ids things.IDPair, p Priority) {
    placement := this.getMaxPlacement() + 1
    statement := `
        INSERT INTO scheduler (fst, snd, placement, priority)
            VALUES
                (   $1,
                    $2,
                    $3,
                    $4);`
    if _, err := this.db.Exec(statement, ids.Fst, ids.Snd, placement, p); err != nil {
        log.Fatal("Error while inserting a request at the back of the queue: ", err)
    }
    return
}

func (this *Scheduler) hasRequest(ids things.IDPair) bool {
    var status bool
    query := `SELECT EXISTS(SELECT * FROM scheduler WHERE (fst = $1 AND snd = $2) OR (fst = $2 AND snd = $1) LIMIT 1)`
    if err := this.db.QueryRow(query, ids.Fst, ids.Snd).Scan(&status); err != nil {
        log.Fatal("Error while trying to determine if the scheduler contains a request: ", err)
    }
    return status
}

func (this *Scheduler) rmRequest(ids things.IDPair) {
    statement := `DELETE FROM scheduler WHERE (fst = $1 AND snd = $2) OR (fst = $2 AND snd = $1)`
    if _, err := this.db.Exec(statement, ids.Fst, ids.Snd); err != nil {
        log.Fatal("Error while trying to delete a scheduler request: ", err)
    }
}

func (this *Scheduler) HasRequest(ids things.IDPair) bool {
    return this.hasRequest(ids)
}

func (this *Scheduler) RequestComparison(ids things.IDPair, p Priority) {
    cmp := 0
    if cmp = things.GetComparison(this.db, ids.Fst, ids.Snd); cmp < 0 {
        cmp = -cmp
    }
    if cmp > this.pointlessThreshold {
        return
    }

    log.Printf("Requesting a comparison for %d, %d\n", int(ids.Fst), int(ids.Snd))
    if p == PLow || p == PMedium || p == PMarginal {
        this.appendRequest(ids, p)
    } else if p == PHigh {
        this.prependRequest(ids, p)
    }

    if !this.hasRequest(ids) {
        log.Fatal(errors.New("something is horribly wrong with the scheduler"))
    }
}

func (this *Scheduler) FillRequest(ids things.IDPair) {
    log.Printf("Filling a request for %d, %d\n", int(ids.Fst), int(ids.Snd))
    this.rmRequest(ids)
    if this.hasRequest(ids) {
        log.Fatal(errors.New("rmRequest doesn't work"))
    }
}

func (this *Scheduler) logPairsWithHeats(user users.User) {
    log.Printf("Pairs with heats:\n")
    query := `
        SELECT
            fst, snd, SUM(heat) AS s_heat
            FROM scheduler, exposure
            WHERE ("user" = $1 AND (image = fst OR image = snd))
            GROUP BY GROUPING SETS ((fst, snd));
    `
    rows, err := this.db.Query(query, user.Id)
    if err != nil {
        log.Fatal("Error querying rows to log: ", err)
    }
    for rows.Next() {
        var fst, snd, heat int
        rows.Scan(&fst, &snd, &heat)
        log.Printf("\t%d, %d, heat = %d\n", fst, snd, heat)
    }
}

func (this *Scheduler) NextRequest(user users.User) *things.IDPair {
    var ids things.IDPair
    var id, placement, emptySum int
    var p Priority

    for !this.hasAnyRequests() {
        runtime.Gosched()
    }

    //this.logPairsWithHeats(user)

    query := `
        SELECT
            id, fst, snd, priority, placement, SUM(heat) AS s_heat
        FROM scheduler, exposure
        WHERE ("user" = $1 AND (image = fst OR image = snd))
        GROUP BY ROLLUP (id, fst, snd, priority, placement)
        ORDER BY
            (s_heat - COALESCE(priority, 0)) ASC,
            placement ASC
        LIMIT 1;
    `
    /*`
        SELECT
            "user", scheduler.id, scheduler.fst, scheduler.snd, exposure.image, exposure.heat
            AS "user", sched_id, fst, snd, image, heat
        FROM scheduler, exposure
        WHERE "user" = $1
        ORDER BY
            SUM(SELECT heat FROM exposure WHERE "user" = $1 AND (image = fst OR image = snd)) ASC,
            placement ASC
        LIMIT 1`*/

    if err := this.db.QueryRow(query, user.Id).Scan(&id, &ids.Fst, &ids.Snd, &p, &placement, &emptySum); err != nil {
        log.Fatal("Error while selecting the highest priority scheduling request: ", err)
        /*query := `
            SELECT id, fst, snd, priority, placement
            FROM scheduler
            WHERE 
        `*/
    }

    log.Printf("Selecting image pair with heat of %d\n", emptySum)

    if p == PLow || p == PMarginal {
        placement = this.getMaxPlacement() + 1
    } else if p == PHigh {
        placement = placement + 1
    } else if p == PMedium {
        placement = placement + 2
    }
    statement := `UPDATE scheduler SET placement = $1 WHERE id = $2`
    if _, err := this.db.Exec(statement, placement, id); err != nil {
        log.Fatal("Error while demoting a scheduling request: ", err)
    }

    return &ids
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
