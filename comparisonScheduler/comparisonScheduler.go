package comparisonScheduler

import (
    "../things")

func RequestComparison(ids things.IDPair, reqs chan things.IDPair, resps chan things.IDPair) {
    reqs <- ids
    <-resps
}

/*type sorter struct {
    reqs chan things.IDPair
    last things.IDPair
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
