package main

import (
    "fmt"
    "github.com/gorilla/mux"
    "net/http"
    "time"
    "log"
    "database/sql"
    "os"
    "runtime"
    _ "github.com/lib/pq"
    "io/ioutil"
    "strconv"
    "github.com/chrismamo1/reflagvsflag/things"
    "github.com/chrismamo1/reflagvsflag/users"
    sched "github.com/chrismamo1/reflagvsflag/comparisonScheduler")

func initDb() *sql.DB {
    dbParams := os.ExpandEnv("user=db_master dbname=reflagvsflag_db sslmode=disable password=${REFLAGVSFLAG_DB_PASSWORD} host=${REFLAGVSFLAG_DB_HOST}")
    db, err := sql.Open("postgres", dbParams)
    if err != nil {
        log.Fatal(err)
    }

    statement := `
    DROP TABLE IF EXISTS images CASCADE;
    DROP TABLE IF EXISTS comparisons CASCADE;
    DROP TABLE IF EXISTS users CASCADE;
    DROP TABLE IF EXISTS exposure CASCADE;
    DROP TABLE IF EXISTS votes;
    DROP TABLE IF EXISTS scheduler;
    `
    _, err = db.Exec(statement)
    if err != nil {
        log.Printf("%q: %s\n", err, statement)
        return nil
    }

    statement = `
    CREATE TABLE IF NOT EXISTS images (
        id SERIAL PRIMARY KEY,
        path TEXT NOT NULL UNIQUE,
        name TEXT,
        description TEXT,
        img_index INT NOT NULL,
        heat INT NOT NULL);
    CREATE TABLE IF NOT EXISTS comparisons (
        "left" INT NOT NULL,
        "right" INT NOT NULL,
        balance INT NOT NULL,
        heat INT NOT NULL,
        FOREIGN KEY ("left") REFERENCES images(id),
        FOREIGN KEY ("right") REFERENCES images(id));
    CREATE TABLE IF NOT EXISTS users (
        id SERIAL PRIMARY KEY,
        ip_addr TEXT NOT NULL UNIQUE);
    CREATE TABLE IF NOT EXISTS exposure (
        "user" INT NOT NULL,
        image INT NOT NULL,
        heat INT NOT NULL,
        FOREIGN KEY ("user") REFERENCES users(id),
        FOREIGN KEY (image) REFERENCES images(id));
    CREATE TABLE IF NOT EXISTS votes (
        id SERIAL PRIMARY KEY,
        "user" INT NOT NULL,
        winner INT NOT NULL,
        loser INT NOT NULL,
        submitted_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT (NOW() AT TIME ZONE 'UTC'),
        FOREIGN KEY ("user") REFERENCES users(id),
        FOREIGN KEY (winner) REFERENCES images(id),
        FOREIGN KEY (loser) REFERENCES images(id),
        CHECK (NOT(winner = loser)));
    CREATE TABLE IF NOT EXISTS scheduler (
        id SERIAL PRIMARY KEY,
        fst INT NOT NULL,
        snd INT NOT NULL,
        placement INT NOT NULL,
        priority INT NOT NULL,
        FOREIGN KEY (fst) REFERENCES images(id),
        FOREIGN KEY (snd) REFERENCES images(id),
        CHECK (fst <> snd));
    TRUNCATE scheduler;
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
    rows, err := db.Query("SELECT id, path, description, img_index, heat FROM images ORDER BY img_index ASC")
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

func VoteHandler(db *sql.DB, scheduler *sched.Scheduler) func(http.ResponseWriter, *http.Request) {
    return func(writer http.ResponseWriter, req *http.Request) {
        var ids things.IDPair
        winner, _ := strconv.Atoi(req.FormValue("winner"))
        loser, _ := strconv.Atoi(req.FormValue("loser"))

        user := users.GetByAddr(db, req.RemoteAddr)
        /*query := `
        SELECT
            CASE WHEN (EXISTS(SELECT * FROM exposure WHERE "user" = $1 AND image = $2)) THEN
                (UPDATE exposure SET heat = heat + 1 WHERE "user" = $3 AND image = $4)
            ELSE
                (INSERT INTO exposure ("user", image, heat) VALUES ($5, $6, 1))
            END
        `
        if _, err := db.Exec(query, user.Id, winner, user.Id, winner, user.Id, winner); err != nil {
            log.Fatal("problem updating/modifying exposure table: ", err)
        }
        if _, err := db.Exec(query, user.Id, loser, user.Id, loser, user.Id, loser); err != nil {
            log.Fatal(err)
        }*/
        user.SubmitVote(db, things.ID(winner), things.ID(loser))

        ids.Fst = things.ID(winner)
        ids.Snd = things.ID(loser)
        query := "SELECT \"left\", \"right\", balance, heat FROM comparisons WHERE ((\"left\" = %d AND \"right\" = %d) OR (\"right\" = %d AND \"left\" = %d))"
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
            query = "INSERT INTO comparisons(\"left\", \"right\", balance, heat) VALUES (%d, %d, %d, %d);"
            query = fmt.Sprintf(query, winner, loser, -1, heat)
        } else {
            query = "UPDATE comparisons SET balance = %d, heat = %d  WHERE \"left\" = %d AND \"right\" = %d;"
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
        scheduler.FillRequest(ids)
    }
}

func RanksHandler(db *sql.DB) func(http.ResponseWriter, *http.Request) {
    return func(writer http.ResponseWriter, req *http.Request) {
        users.GetByAddr(db, req.RemoteAddr)

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

func UsersHandler(db *sql.DB) func(http.ResponseWriter, *http.Request) {
    return func(writer http.ResponseWriter, req *http.Request) {
        users.GetByAddr(db, req.RemoteAddr)

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

        allUsers := users.GetAll(db)

        for i := 0; i < len(allUsers); i = i + 1 {
            els += fmt.Sprintf("<li>%s</li>", allUsers[i].Render(db, "reFlagVsFlag_user"))
        }

        page = fmt.Sprintf(page, els);

        writer.Write([]byte(page))
    }
}

func JudgeHandler(db *sql.DB, scheduler *sched.Scheduler) func(http.ResponseWriter, *http.Request) {
    return func(writer http.ResponseWriter, req *http.Request) {
        var ids *things.IDPair
        for ids == nil {
            ids = scheduler.NextRequest(*users.GetByAddr(db, req.RemoteAddr))
        }

        bumpExposure := func(user *users.User, img things.ID) {
            var exists bool
            query := `SELECT (EXISTS(SELECT * FROM exposure WHERE "user" = $1 AND image = $2))`
            err := db.QueryRow(query, user.Id, img).Scan(&exists)
            if err != nil {
                log.Fatal(err)
            }
            if exists {
                query := `UPDATE exposure SET heat = heat + 1 WHERE "user" = $1 AND image = $2`
                if _, err := db.Exec(query, user.Id, img); err != nil {
                    log.Fatal(err)
                }
            } else {
                query := `INSERT INTO exposure ("user", image, heat) VALUES ($1, $2, 1)`
                if _, err := db.Exec(query, user.Id, img); err != nil {
                    log.Fatal(err)
                }
            }
        }

        user := users.GetByAddr(db, req.RemoteAddr)

        bumpExposure(user, ids.Fst)
        bumpExposure(user, ids.Snd)

        left, right := things.SelectImages(db, *ids)
        page := `
        <h1>Which of these flags is better?</h1>
        <a href="/vote?winner=%d&loser=%d">
        %s
        </a>
        <a href="/vote?winner=%d&loser=%d">
        %s
        </a>
        <div />
        <a href="/ranks">Click here to see the current rankings</a>
        `
        page = fmt.Sprintf(page, left.Id, right.Id, things.RenderNormal(left), right.Id, left.Id, things.RenderNormal(right))
        writer.Write([]byte(page))
    }
}

func IndexHandler(writer http.ResponseWriter, req *http.Request) {
    writer.Header().Add("Location", "/judge")
    writer.WriteHeader(302)
    page := `
        <h1>Thanks for voting!</h1>
    `
    writer.Write([]byte(page))
    return
}

func ShutdownHandler(srv *http.Server, db *sql.DB) func(http.ResponseWriter, *http.Request) {
    return func(writer http.ResponseWriter, req *http.Request) {
        defer db.Close()
        defer srv.Shutdown(req.Context())
        writer.Write([]byte("Shutting down the server."))
    }
}

func refreshImages(db *sql.DB) {
    files, err := ioutil.ReadDir("./static/flags")
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

        query := `INSERT INTO images (name, path, img_index, heat, description) VALUES ($1, $2, $3, 0, '')`

        path, err := ioutil.ReadFile("./static/flags/" + file.Name())
        if err != nil {
            fmt.Printf("Couldn't open the file \"%s\":\n")
            log.Fatal(err)
        }

        _, err = tx.Exec(query, file.Name(), string(path), max + 1)
        if err != nil {
            fmt.Printf("problem encountered while trying to run the query \"%s\":\n", query)
            fmt.Printf("(used values: \"%s\", \"%s\", %d)\n", file.Name(), string(path), max + 1)
            log.Print(err)
        }

        err = tx.Commit()

        if err != nil {
            fmt.Println("Found a non-fatal problem adding an image")
            // duplicate image
        }
    }
}

func flagSort(db *sql.DB, scheduler *sched.Scheduler) {
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

        iCenter := (iLeft + iRight) / 2

        refreshAll := func(iLeft int, iPivot int, iRight int) (things.ID, things.ID, things.ID) {
                var left, pivot, right things.ID

                if err := db.QueryRow("SELECT id FROM images WHERE img_index = $1", iLeft).Scan(&left); err != nil {
                    log.Fatal(err)
                }
                if err := db.QueryRow("SELECT id FROM images WHERE img_index = $1", iPivot).Scan(&pivot); err != nil {
                    log.Fatal(err)
                }
                if err := db.QueryRow("SELECT id FROM images WHERE img_index = $1", iRight).Scan(&right); err != nil {
                    log.Fatal(err)
                }

                return left, pivot, right
        }

        queueUpComparisons := func(iLeft int, iRight int) {
            l := iLeft
            iPivot := (iLeft + iRight) / 2
            _, oldPivot, oldRight := refreshAll(iLeft, iRight, iPivot)
            for iLeft < iRight {
                if iLeft == iPivot {
                    iLeft = iLeft + 1
                    continue
                }
                left, pivot, right := refreshAll(iLeft, iPivot, iRight)
                if right != oldRight || pivot != oldPivot {
                    iLeft = l
                }
                request := things.IDPair{Fst: left, Snd: pivot}
                scheduler.RequestComparison(request, sched.PMarginal)
                for scheduler.HasRequest(request) {
                    // no-op
                    runtime.Gosched()
                }
                iLeft = iLeft + 1
            }
        }

        go queueUpComparisons(iLeft, iCenter)
        go queueUpComparisons(iCenter + 1, iRight)

        // wait
        <-readyToStart

        fmt.Printf("\tDoing quickSort(\"left\" = %d, \"right\" = %d, done = <chan bool>\n", iLeft, iRight)

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
            err = tx.QueryRow("SELECT img_index FROM images WHERE id = $1", a).Scan(&iA)
            if err != nil {
                log.Fatal(err)
            }
            err = tx.QueryRow("SELECT img_index FROM images WHERE id = $1", b).Scan(&iB)
            if err != nil {
                log.Fatal(err)
            }
            _, err = tx.Exec("UPDATE images SET img_index = $1 WHERE id = $2", iB, a)
            if err != nil {
                log.Fatal(err)
            }
            _, err = tx.Exec("UPDATE images SET img_index = $1 WHERE id = $2", iA, b)
            if err != nil {
                log.Fatal(err)
            }
            tx.Commit()
        }

        startedLeft := false
        startedRight := false

        readyForLeft := make(chan bool)
        readyForRight := make(chan bool)

        isDoneLeft := make(chan bool)
        go quickSort(iLeft, iCenter, readyForLeft, isDoneLeft)

        isDoneRight := make(chan bool)
        go quickSort(iCenter + 1, iRight, readyForRight, isDoneRight)

        var left, right, pivot things.ID
        err := db.QueryRow("SELECT id FROM images WHERE img_index = $1", iLeft).Scan(&left)
        if err != nil {
            log.Fatal(err)
        }
        err = db.QueryRow("SELECT id FROM images WHERE img_index = $1", iRight).Scan(&right)
        if err != nil {
            log.Fatal(err)
        }
        err = db.QueryRow("SELECT id FROM images WHERE img_index = $1", iCenter).Scan(&pivot)
        if err != nil {
            log.Fatal(err)
        }

        l := iLeft
        r := iRight

        for true {
            if left == pivot {
                // jump over the pivot
                l = l + 1
                err = db.QueryRow("SELECT id FROM images WHERE img_index = $1", l).Scan(&left)
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
            for cmp * cmp < 1 {
                fmt.Printf("Need a stronger comparison for %d and %d\n", request.Fst, request.Snd)
                scheduler.RequestComparison(request, sched.PHigh)
                for scheduler.HasRequest(request) {
                    // no-op
                    runtime.Gosched()
                }
                cmp = things.GetComparison(db, left, pivot)
            }

            // if we're above the pivot of the left child, start working on its comparisons
            if l > (iLeft + iCenter) / 2 && !startedLeft {
                fmt.Println("Working on the left child's comparisons preemptively...")
                var newPivot, rando things.ID
                iPivot := (iLeft + iCenter) / 2
                err := db.QueryRow("SELECT id FROM images WHERE img_index = $1", iPivot).Scan(&newPivot)
                if err != nil {
                    log.Fatal(err)
                }
                query := `
                SELECT id
                FROM images
                WHERE img_index >= $1 AND img_index < $2 AND img_index < $3 AND img_index != $4
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

                scheduler.RequestComparison(request, sched.PMedium)
            }
            // if we're below the pivot of the right child, start working on its comparisons
            if r < (iRight + (iCenter + 1)) / 2 && !startedRight {
                fmt.Println("Working on the right child's comparisons preemptively...")
                var newPivot, rando things.ID
                iPivot := (iRight + (iCenter + 1)) / 2
                err := db.QueryRow("SELECT id FROM images WHERE img_index = $1", iPivot).Scan(&newPivot)
                if err != nil {
                    log.Fatal(err)
                }
                query := `
                SELECT id
                FROM images
                WHERE img_index > $1 AND img_index <= $2 AND img_index != $3
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

                scheduler.RequestComparison(request, sched.PMedium)
            }

            if cmp >= 0 { // images[left] > images[pivot]
                swapIndices(left, right)

                r = r - 1
                // update the new id's
                err = db.QueryRow("SELECT id FROM images WHERE img_index = $1", r).Scan(&right)
                if err != nil {
                    log.Fatal(err)
                }
                err = db.QueryRow("SELECT id FROM images WHERE img_index = $1", l).Scan(&left)
                if err != nil {
                    log.Fatal(err)
                }
                //fmt.Println("Succesfully did a move in flagSort")
            } else {
                l = l + 1
                // select the new left from l
                err = db.QueryRow("SELECT id FROM images WHERE img_index = $1", l).Scan(&left)
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

    flagSort(db, scheduler)
    return
}

func main() {
    fmt.Println("About to initialize the database")
    db := initDb()
    defer fmt.Println("Closing shit")
    defer db.Close()

    scheduler := sched.Make(db)

    //imageComparisonRequests := make(chan things.IDPair)
    //imageComparisonResponses := make(chan things.IDPair)

    fmt.Println("About to refresh images")
    refreshImages(db)

    fmt.Println("About to create the mux")
    r := mux.NewRouter()

    srv := &http.Server{
        Handler:        r,
        Addr:           "172.31.76.179:80",
        WriteTimeout:   15 * time.Second,
        ReadTimeout:    15 * time.Second,
    }

    r.HandleFunc("/index", IndexHandler)
    r.HandleFunc("/index.html", IndexHandler)
    r.HandleFunc("/", IndexHandler)
    r.HandleFunc("/ranks", RanksHandler(db))
    r.HandleFunc("/users", UsersHandler(db))
    r.HandleFunc("/judge", JudgeHandler(db, scheduler))
    r.HandleFunc("/vote", VoteHandler(db, scheduler))
    r.HandleFunc("/shutdown", ShutdownHandler(srv, db))
    r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("static/"))))

    go flagSort(db, scheduler)

    fmt.Println("About to ListenAndServe")
    srv.ListenAndServe()
}
