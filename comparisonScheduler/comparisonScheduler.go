package comparisonScheduler

import (
    "sync"
    "github.com/chrismamo1/reflagvsflag/things")

type node struct {
    x things.IDPair
    next *node
}

type Scheduler struct {
    requests *node
    //satisfactions *node
    mux sync.Mutex
}

func (this *Scheduler) appendRequest(ids things.IDPair) {
    if this.requests == nil {
        this.requests = &node{x: ids, next: nil}
    }
    var n *node
    for n = this.requests; n.next != nil; n = n.next {
        // no-op
    }
    n.next = &node{x: ids, next: nil}
    return
}

func (this *Scheduler) addRequest(ids things.IDPair) {
    n := &node{x: ids, next: this.requests}
    this.requests = n
}

func (this *Scheduler) rmRequest(ids things.IDPair) {
    for n := this.requests; n != nil; n = n.next {
        if n.x.Equivalent(ids) {
            *n = *(n.next)
            return
        }
    }
}

func (this *Scheduler) HasRequest(ids things.IDPair) bool {
    for n := this.requests; n != nil; n = n.next {
        if n.x.Equivalent(ids) {
            return true
        }
    }
    return false
}

func (this *Scheduler) RequestComparison(ids things.IDPair) {
    this.mux.Lock()
    defer this.mux.Unlock()

    if !this.HasRequest(ids) {
        this.addRequest(ids)
    }
}

func (this *Scheduler) FillRequest(ids things.IDPair) {
    this.mux.Lock()
    defer this.mux.Unlock()

    if this.HasRequest(ids) {
        this.rmRequest(ids)
    }
}

func (this *Scheduler) NextRequest() *things.IDPair {
    this.mux.Lock()
    defer this.mux.Unlock()

    if this.requests == nil {
        return nil
    }

    ids := this.requests.x
    this.rmRequest(ids)
    this.appendRequest(ids)

    return &ids
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
