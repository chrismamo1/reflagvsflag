package main

import (
    "fmt"
    "github.com/gorilla/mux"
    "net/http"
    "time"
    "log"
    "database/sql"
    _ "github.com/mattn/go-sqlite3"
    "io/ioutil"
    "strconv"
    "./things"
    scheduler "./comparisonScheduler")

func initDb() *sql.DB {
    db, err := sql.Open("sqlite3", "./CuterThing.db")
    if err != nil {
        log.Fatal(err)
    }

    statement := `
    CREATE TABLE IF NOT EXISTS images (
        id INTEGER PRIMARY KEY,
        path TEXT NOT NULL UNIQUE,
        desc TEXT,
        img_index INT NOT NULL,
        heat INT NOT NULL);
    CREATE TABLE IF NOT EXISTS comparisons (
        left INT NOT NULL,
        right INT NOT NULL,
        balance INT NOT NULL,
        heat INT NOT NULL,
        FOREIGN KEY (left) REFERENCES images(id),
        FOREIGN KEY (right) REFERENCES images(id));
    CREATE TABLE IF NOT EXISTS users (
        id INTEGER PRIMARY KEY,
        ip_addr TEXT NOT NULL);
    CREATE TABLE IF NOT EXISTS exposure (
        user INT NOT NULL,
        image INT NOT NULL,
        heat INT NOT NULL,
        FOREIGN KEY (user) REFERENCES users(id),
        FOREIGN KEY (image) REFERENCES images(id));
    `
    _, err = db.Exec(statement)
    if err != nil {
        log.Printf("%q: %s\n", err, statement)
        return nil
    }

    fmt.Println("initialized database succesfully")
    return db
}

func loadImageStore(db *sql.DB) []things.Thing {
    rows, err := db.Query("SELECT id, path, desc, img_index, heat FROM images ORDER BY img_index ASC")
    if err != nil {
        log.Fatal(err)
    }
    defer rows.Close()

    var imageStore []things.Thing

    for rows.Next() {
        var img things.Thing
        err = rows.Scan(&img.Id, &img.Path, &img.Desc, &img.Index, &img.Heat)
        if err != nil {
            log.Fatal(err)
        }
        imageStore = append(imageStore, img)
    }
    return imageStore
}

func VoteHandler(db *sql.DB, resps chan things.IDPair) func(http.ResponseWriter, *http.Request) {
    return func(writer http.ResponseWriter, req *http.Request) {
        var ids things.IDPair
        winner, _ := strconv.Atoi(req.FormValue("winner"))
        loser, _ := strconv.Atoi(req.FormValue("loser"))
        ids.Fst = things.ID(winner)
        ids.Snd = things.ID(loser)
        query := "SELECT left, right, balance, heat FROM comparisons WHERE ((left = %d AND right = %d) OR (right = %d AND left = %d))"
        query = fmt.Sprintf(query, winner, loser, winner, loser)
        rows, err := db.Query(query)
        if err != nil {
            log.Fatal(err)
        }
        defer rows.Close()

        nrows := 0

        var left,right,balance,heat int
        for rows.Next() {
            err = rows.Scan(&left, &right, &balance, &heat)
            if err != nil {
                log.Fatal(err)
            }
            if left == winner {
                balance = balance - 1
            } else if right == winner {
                balance = balance + 1
            } else {
                fmt.Println("sql query fucked up badly")
                nrows = 0
                break
            }
            heat = heat + 1
            nrows = nrows + 1
        }
        if (nrows == 0) {
            query = "INSERT INTO comparisons(left, right, balance, heat) VALUES (%d, %d, %d, %d);"
            query = fmt.Sprintf(query, winner, loser, -1, heat)
        } else {
            query = "UPDATE comparisons SET balance = %d, heat = %d  WHERE left = %d AND right = %d;"
            query = fmt.Sprintf(query, balance, heat, left, right)
        }
        _, err = db.Exec(query)
        if err != nil {
            log.Fatal(err)
        }
        writer.Header().Add("Location", "/judge")
        writer.WriteHeader(302)
        page := `
        <h1>Thanks for voting!</h1>
        `
        writer.Write([]byte(page))
        resps <- ids
    }
}

func RanksHandler(db *sql.DB) func(http.ResponseWriter, *http.Request) {
    return func(writer http.ResponseWriter, req *http.Request) {
        store := loadImageStore(db)

        page := `
        <html>
            <head></head>
            <body>
                <ol>
                    %s
                </ol>
            </body>
        </html>
        `

        els := ""

        for i := 0; i < len(store); i = i + 1 {
            els += fmt.Sprintf("<li>%s</li>", things.RenderSmall(store[i]))
        }

        page = fmt.Sprintf(page, els);

        writer.Write([]byte(page))
    }
}

func JudgeHandler(db *sql.DB, reqs chan things.IDPair, resps chan things.IDPair) func(http.ResponseWriter, *http.Request) {
    return func(writer http.ResponseWriter, req *http.Request) {
        var ids things.IDPair
RETRY:
        select {
        case some_ids := <-reqs:
            ids = some_ids
        default:
            /*var some_ids things.IDPair
            fmt.Println("JudgeHandler is responding with a -1 pair")
            some_ids.Fst = -1
            some_ids.Snd = -1
            fmt.Println("-1 pair about to send")
            resps <- some_ids
            fmt.Println("-1 pair sent")*/
            goto RETRY
        }
        left, right := things.SelectImages(db, ids)
        page := `
        <h1>Which of these flags is better?</h1>
        <a href="/vote?winner=%d&loser=%d">
        %s
        </a>
        <a href="/vote?winner=%d&loser=%d">
        %s
        </a>
        `
        page = fmt.Sprintf(page, left.Id, right.Id, things.RenderNormal(left), right.Id, left.Id, things.RenderNormal(right))
        writer.Write([]byte(page))
    }
}

func ShutdownHandler(srv *http.Server, db *sql.DB) func(http.ResponseWriter, *http.Request) {
    return func(writer http.ResponseWriter, req *http.Request) {
        defer db.Close()
        defer srv.Shutdown(req.Context())
        writer.Write([]byte("Shutting down the server."))
    }
}

func refreshImages(db *sql.DB) {
    files, err := ioutil.ReadDir("./static/img")
    if err != nil {
        log.Fatal(err)
    }

    for _, file := range files {
        tx, err := db.Begin()
        if err != nil {
            log.Fatal(err)
        }
        var max int
        err = tx.QueryRow("SELECT MAX(img_index) FROM images").Scan(&max)
        if err != nil {
            // 
            max = 0
        }
        statement, err := tx.Prepare("INSERT INTO images(path,desc,img_index,heat) VALUES (?, '', ?, 0);")
        if err != nil {
            log.Fatal(err)
        }

        _, err = statement.Exec(file.Name(), max + 1)

        tx.Commit()
        statement.Close()

        if err != nil {
            // duplicate image
        }
    }
}

func flagSort(db *sql.DB, req chan things.IDPair, resp chan things.IDPair) {
    var left, pivot, right int

    err := db.QueryRow("SELECT img_index FROM images ORDER BY img_index ASC LIMIT 1").Scan(&left)
    if err != nil {
        log.Fatal(err)
    }

    err = db.QueryRow("SELECT images.img_index FROM images ORDER BY img_index ASC LIMIT 1 OFFSET ((SELECT COUNT(*) FROM IMAGES) / 2)").Scan(&pivot)
    if err != nil {
        log.Fatal(err)
    }

    err = db.QueryRow("SELECT images.img_index FROM images ORDER BY img_index ASC LIMIT 1 OFFSET ((SELECT count(*) FROM images) - 1)").Scan(&right)
    if err != nil {
        log.Fatal(err)
    }

    var quickSort func(int, int, chan bool, chan bool)

    // iLeft = img_index of the left-hand boundary (inclusive)
    // iRight = img_index of the right-hand boundary (inclusive)
    // readyToStart = a channel which will receive [true] then immediately close when the parent
    //                iteration is done with elements from iLeft to iRight
    // done = quickSort will send [true] over this channel when it's done processing all of its
    //        elements
    quickSort = func(iLeft int, iRight int, readyToStart chan bool, done chan bool) {
        finalize := func() {
            fmt.Println("Done with a QuickSort iteration")
            done <- true
        }
        defer finalize()

        // wait
        <-readyToStart

        fmt.Printf("\tDoing quickSort(left = %d, right = %d, done = <chan bool>\n", iLeft, iRight)

        if iLeft >= iRight {
            // handle a couple of edge-cases
            ///done <- true
            return
        }

        /** atomically swap the indices of two images, given their ID's */
        swapIndices := func(a things.ID, b things.ID) {
            tx, err := db.Begin()
            if err != nil {
                log.Fatal(err)
            }
            var iA, iB int
            err = tx.QueryRow("SELECT img_index FROM images WHERE id = ?", a).Scan(&iA)
            if err != nil {
                log.Fatal(err)
            }
            err = tx.QueryRow("SELECT img_index FROM images WHERE id = ?", b).Scan(&iB)
            if err != nil {
                log.Fatal(err)
            }
            _, err = tx.Exec("UPDATE images SET img_index = ? WHERE id = ?", iB, a)
            if err != nil {
                log.Fatal(err)
            }
            _, err = tx.Exec("UPDATE images SET img_index = ? WHERE id = ?", iA, b)
            if err != nil {
                log.Fatal(err)
            }
            tx.Commit()
        }

        startedLeft := false
        startedRight := false

        readyForLeft := make(chan bool)
        readyForRight := make(chan bool)

        iCenter := (iLeft + iRight) / 2

        isDoneLeft := make(chan bool)
        go quickSort(iLeft, iCenter, readyForLeft, isDoneLeft)

        isDoneRight := make(chan bool)
        go quickSort(iCenter + 1, iRight, readyForRight, isDoneRight)

        var left, right, pivot things.ID
        err := db.QueryRow("SELECT id FROM images WHERE img_index = ?", iLeft).Scan(&left)
        if err != nil {
            log.Fatal(err)
        }
        err = db.QueryRow("SELECT id FROM images WHERE img_index = ?", iRight).Scan(&right)
        if err != nil {
            log.Fatal(err)
        }
        err = db.QueryRow("SELECT id FROM images WHERE img_index = ?", iCenter).Scan(&pivot)
        if err != nil {
            log.Fatal(err)
        }

        l := iLeft
        r := iRight

        for true {
            if left == pivot {
                // jump over the pivot
                l = l + 1
                err = db.QueryRow("SELECT id FROM images WHERE img_index = ?", l).Scan(&left)
                if err != nil {
                    fmt.Printf("Error in moving left over the pivot when l = %d, message: ", l)
                    return
                }
            }

            var request things.IDPair
            request.Fst = left
            request.Snd = pivot


            cmp := things.GetComparison(db, left, pivot)

            fmt.Printf("\tMight need a stronger comparison for %d and %d\n", request.Fst, request.Snd)
            for cmp * cmp < 4 {
                random := things.GetRandomPair(db)
                go scheduler.RequestComparison(random, req, resp)
                fmt.Printf("Need a stronger comparison for %d and %d\n", request.Fst, request.Snd)
                scheduler.RequestComparison(request, req, resp)
                cmp = things.GetComparison(db, left, pivot)
            }

            // if we're above the pivot of the left child, start working on its comparisons
            if l > (iLeft + iCenter) / 2 && !startedLeft {
                fmt.Println("Working on the left child's comparisons preemptively...")
                var newPivot, rando things.ID
                iPivot := (iLeft + iCenter) / 2
                err := db.QueryRow("SELECT id FROM images WHERE img_index = ?", iPivot).Scan(&newPivot)
                if err != nil {
                    log.Fatal(err)
                }
                query := `
                SELECT id
                FROM images
                WHERE img_index >= ? AND img_index < ? AND img_index < ? AND img_index != ?
                ORDER BY RANDOM()
                LIMIT 1
                `
                err = db.QueryRow(query, iLeft, l, iCenter, iPivot).Scan(&rando)
                if err != nil {
                    log.Fatal(err)
                }

                var request things.IDPair
                request.Fst = newPivot
                request.Snd = rando

                scheduler.RequestComparison(request, req, resp)
            }
            // if we're below the pivot of the right child, start working on its comparisons
            if r < (iRight + (iCenter + 1)) / 2 && !startedRight {
                fmt.Println("Working on the right child's comparisons preemptively...")
                var newPivot, rando things.ID
                iPivot := (iRight + (iCenter + 1)) / 2
                err := db.QueryRow("SELECT id FROM images WHERE img_index = ?", iPivot).Scan(&newPivot)
                if err != nil {
                    log.Fatal(err)
                }
                query := `
                SELECT id
                FROM images
                WHERE img_index > ? AND img_index <= ? AND img_index != ?
                ORDER BY RANDOM()
                LIMIT 1
                `
                err = db.QueryRow(query, r, iRight, iPivot).Scan(&rando)
                if err != nil {
                    log.Fatal(err)
                }

                var request things.IDPair
                request.Fst = newPivot
                request.Snd = rando

                scheduler.RequestComparison(request, req, resp)
            }

            if cmp >= 0 { // images[left] > images[pivot]
                swapIndices(left, right)

                r = r - 1
                // update the new id's
                err = db.QueryRow("SELECT id FROM images WHERE img_index = ?", r).Scan(&right)
                if err != nil {
                    log.Fatal(err)
                }
                err = db.QueryRow("SELECT id FROM images WHERE img_index = ?", l).Scan(&left)
                if err != nil {
                    log.Fatal(err)
                }
                //fmt.Println("Succesfully did a move in flagSort")
            } else {
                l = l + 1
                // select the new left from l
                err = db.QueryRow("SELECT id FROM images WHERE img_index = ?", l).Scan(&left)
            }

            if l > iCenter && !startedLeft {
                fmt.Println("\tStarting left child")
                readyForLeft <- true
                startedLeft = true
            }
            if r <= iCenter && !startedRight {
                fmt.Println("\tStarting right child")
                readyForRight <- true
                startedRight = true
            }

            if l >= r {
                if !startedLeft {
                    fmt.Println("\tStarting left child")
                    readyForLeft <- true
                }
                if !startedRight {
                    fmt.Println("\tStarting right child")
                    readyForRight <- true
                }
                fmt.Println("Basically done with a quickSort iteration; now just waiting on kids")
                // I don't really care about the results, I just need to wait for these to finish executing
                <-isDoneLeft
                <-isDoneRight

                return
            }
        }
    }

    done := make(chan bool)
    readyToStart := make(chan bool)

    go quickSort(left, right, readyToStart, done)

    readyToStart <- true
    isDone := <-done

    if isDone {
        fmt.Println("\tDone sorting images!")
    } else {
        fmt.Println("\tisDone reported something fucky")
    }

    flagSort(db, req, resp)
    return
}

func main() {
    db := initDb()
    defer fmt.Println("Closing shit")
    defer db.Close()

    imageComparisonRequests := make(chan things.IDPair)
    imageComparisonResponses := make(chan things.IDPair)

    refreshImages(db)

    r := mux.NewRouter()

    srv := &http.Server{
        Handler:        r,
        Addr:           "127.0.0.1:3456",
        WriteTimeout:   15 * time.Second,
        ReadTimeout:    15 * time.Second,
    }

    r.HandleFunc("/ranks", RanksHandler(db))
    r.HandleFunc("/judge", JudgeHandler(db, imageComparisonRequests, imageComparisonResponses))
    r.HandleFunc("/vote", VoteHandler(db, imageComparisonResponses))
    r.HandleFunc("/shutdown", ShutdownHandler(srv, db))
    r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("static/"))))

    go flagSort(db, imageComparisonRequests, imageComparisonResponses)

    srv.ListenAndServe()
}
