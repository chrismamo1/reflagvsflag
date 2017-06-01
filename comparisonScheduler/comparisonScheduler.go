package comparisonScheduler

import (
    "database/sql"
    "errors"
    "log"
    "github.com/chrismamo1/reflagvsflag/things"
    _ "github.com/lib/pq")

type Scheduler struct {
    db *sql.DB
}

func (this *Scheduler) appendRequest(ids things.IDPair) {
    statement := `INSERT INTO scheduler (fst, snd, placement) VALUES ($1, $2, ((SELECT MAX(placement)) + 1))`
    if _, err := this.db.Exec(statement, ids.Fst, ids.Snd); err != nil {
        log.Fatal("Error while requesting a comparison: ", err)
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

func (this *Scheduler) RequestComparison(ids things.IDPair) {
    this.appendRequest(ids)

    if !this.hasRequest(ids) {
        log.Fatal(errors.New("something is horribly wrong with the scheduler"))
    }
}

func (this *Scheduler) FillRequest(ids things.IDPair) {
    if this.hasRequest(ids) {
        this.rmRequest(ids)
        if this.hasRequest(ids) {
            log.Fatal(errors.New("rmRequest doesn't work"))
        }
    }
}

func (this *Scheduler) NextRequest() *things.IDPair {
    var ids things.IDPair
    var id int

    query := "SELECT id, fst, snd FROM scheduler ORDER BY placement ASC LIMIT 1"

    if err := this.db.QueryRow(query).Scan(&id, &ids.Fst, &ids.Snd); err != nil {
        log.Fatal("Error while selecting the highest priority scheduling request: ", err)
    }

    statement := `UPDATE scheduler SET placement = ((SELECT MAX(placement)) + 1) WHERE id = $1`
    if _, err := this.db.Exec(statement, id); err != nil {
        log.Fatal("Error while moving a scheduling request to the back of the queue: ", err)
    }

    return &ids
}

func Make(db *sql.DB) *Scheduler {
    statement := `
        CREATE TEMPORARY TABLE scheduler (
            id SERIAL PRIMARY KEY,
            fst INT NOT NULL,
            snd INT NOT NULL,
            placement INT NOT NULL,
            CHECK (fst <> snd),
            CHECK (EXISTS (SELECT * FROM images WHERE id = fst LIMIT 1)),
            CHECK (EXISTS (SELECT * FROM images WHERE id = snd LIMIT 1))
        );
    `

    if _, err := db.Exec(statement); err != nil {
        log.Fatal("Error while making the temporary scheduling table: ", err)
    }

    return &Scheduler{db: db}
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
