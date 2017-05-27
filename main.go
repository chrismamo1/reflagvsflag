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
    "things")

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
    rows, err := db.Query("SELECT id, path, desc, img_index, heat FROM images ORDER BY img_index DESC")
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
        fmt.Printf("setting heat to %d\n", heat)
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

func JudgeHandler(db *sql.DB, reqs chan things.IDPair) func(http.ResponseWriter, *http.Request) {
    return func(writer http.ResponseWriter, req *http.Request) {
        fmt.Println("JudgeHandler is about to wait for some ID's")
        ids := <-reqs
        fmt.Println("JudgeHandler got what it was waiting for")
        left, right := things.SelectImages(db, ids)
        page := `
        <h1>Which of these things is cuter?</h1>
        <a href="/vote?winner=%d&loser=%d">
        %s
        </a>
        <a href="/vote?winner=%d&loser=%d">
        %s
        </a>
        `
        page = fmt.Sprintf(page, left.Id, right.Id, things.RenderNormal(left), right.Id, left.Id, things.RenderNormal(right))
        fmt.Println("About to write a page from JudgeHandler...")
        writer.Write([]byte(page))
    }
}

func ShutdownHandler(srv *http.Server, db *sql.DB) func(http.ResponseWriter, *http.Request) {
    return func(writer http.ResponseWriter, req *http.Request) {
        writer.Write([]byte("Shutting down the server."))
        db.Close()
        srv.Shutdown(req.Context())
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

        fmt.Println("Found an image...\n")
        res, err := statement.Exec(file.Name(), max + 1)

        tx.Commit()
        statement.Close()

        if err != nil {
            // duplicate image
        } else {
            tx, err := db.Begin()
            if err != nil {
                log.Fatal(err)
            }
            statement, err := tx.Prepare("INSERT INTO comparisons(left, right, balance, heat) VALUES (?, ?, 0, 0);")
            if err != nil {
                log.Fatal(err)
            }
            defer statement.Close()
            id, _ := res.LastInsertId()
            for i := 1; i < int(id); i = i + 1 {
                _, err := statement.Exec(id, i)
                if err != nil {
                    fmt.Print("error adding a comparison on a new image")
                    log.Fatal(err)
                }
                fmt.Println("Successfully added an empty comparison on a new image")
            }
            tx.Commit()
        }
    }
}

func flagSort(db *sql.DB, req chan things.IDPair, resp chan things.IDPair) {
    var left, pivot, right things.ID

    err := db.QueryRow("SELECT id FROM images ORDER BY img_index ASC LIMIT 1").Scan(&left)
    if err != nil {
        log.Fatal(err)
    }

    err = db.QueryRow("SELECT images.id FROM images ORDER BY img_index ASC LIMIT 1 OFFSET ((SELECT COUNT(*) FROM IMAGES) / 2)").Scan(&pivot)
    if err != nil {
        log.Fatal(err)
    }

    err = db.QueryRow("SELECT images.id FROM images ORDER BY img_index ASC LIMIT 1 OFFSET ((SELECT count(*) FROM images) - 1)").Scan(&right)
    if err != nil {
        log.Fatal(err)
    }

    /**
    * @return nil if abs(iRight - iLeft) < 1, the ID of the pivot element between them otherwise
    */
    selectPivot := func(iLeft int, iRight int) things.ID {
        fmt.Println("Selecting a pivot.")
        diff := iRight - iLeft
        if diff == 0 {
            return -1
        } else {
            var id things.ID
            i := int((float64(iLeft) + float64(iRight) + 0.5) / 2.0)
            err := db.QueryRow("SELECT id FROM images WHERE img_index = ?", i).Scan(&id)
            if err != nil {
                log.Fatal(err)
            }
            return id
        }
    }

    var quickSort func(things.ID, things.ID, things.ID, chan bool)

    quickSort = func(left things.ID, pivot things.ID, right things.ID, done chan bool) {
        var iLeft, iRight int
        err := db.QueryRow("SELECT img_index FROM images WHERE id = ?", left).Scan(&iLeft)
        if err != nil {
            log.Fatal(err)
        }
        err = db.QueryRow("SELECT img_index FROM images WHERE id = ?", right).Scan(&iRight)
        if err != nil {
            log.Fatal(err)
        }
        l := iLeft
        r := iRight
        if iLeft > iRight {
            done <- true
            return
        }
        if left == right {
            done <- true
            return
        }
        for true {
            if left == pivot {
                l = l + 1
                err = db.QueryRow("SELECT id FROM images WHERE img_index = ?", l).Scan(&left)
                if err != nil {
                    fmt.Printf("Error in moving left over the pivot when l = %d, message: ", l)
                    done <- true
                    return
                }
            }
            var request things.IDPair
            request.Fst = left
            request.Snd = pivot
            RETRY:
            fmt.Printf("Requesting images left %d and pivot %d (right = %d)\n", int(left), int(pivot), int(right))
            req <- request
            fmt.Println("Request went through, now waiting for a response")
            ids := <-resp
            fmt.Println("Response received")
            isValid := (ids.Fst == request.Fst && ids.Snd == request.Snd)
            isValid = isValid || (ids.Fst == request.Snd && ids.Snd == request.Fst)
            if !isValid {
                fmt.Println("Invalid response, trying again")
                goto RETRY
            }
            cmp := things.GetComparison(db, left, pivot)
            if cmp <= 0 { // images[left] > images[pivot]
                // swap index of images[left] with images[right]
                _, err = db.Exec("UPDATE images SET img_index = ? WHERE id = ?", r, left)
                if err != nil {
                    log.Fatal(err)
                }
                _, err = db.Exec("UPDATE images SET img_index = ? WHERE id = ?", l, right)
                if err != nil {
                    log.Fatal(err)
                }
                // move r down
                r = r - 1
                // select the new right from r
                err = db.QueryRow("SELECT id FROM images WHERE img_index = ?", r).Scan(&right)
                err = db.QueryRow("SELECT id FROM images WHERE img_index = ?", l).Scan(&left)
                fmt.Println("Succesfully did a move in flagSort")
            } else {
                l = l + 1
                // select the new left from l
                err = db.QueryRow("SELECT id FROM images WHERE img_index = ?", l).Scan(&left)
            }

            if l > r {
                newPivot := selectPivot(iLeft, iRight)
                if newPivot == -1 || iLeft == iRight || iRight - iLeft <= 1 {
                    done <- true
                    return
                } else {
                    iCenter := (iLeft + iRight) / 2

                    isDoneLeft := make(chan bool)
                    err = db.QueryRow("SELECT id FROM images WHERE img_index = ?", iLeft).Scan(&left)
                    db.QueryRow("SELECT id FROM images WHERE img_index = ?", iCenter).Scan(&pivot)
                    go quickSort(left, selectPivot(iLeft, iCenter), pivot, isDoneLeft)

                    isDoneRight := make(chan bool)
                    db.QueryRow("SELECT id FROM images WHERE img_index = ?", iCenter + 1).Scan(&left)
                    db.QueryRow("SELECT id FROM images WHERE img_index = ?", iRight).Scan(&right)
                    go quickSort(left, selectPivot(iCenter + 1, iRight), right, isDoneRight)

                    // I don't really care about the results, I just need to wait for these to finish executing
                    <-isDoneLeft
                    <-isDoneRight

                    done <- true
                    return
                }
            }
        }
    }

    done := make(chan bool)

    go quickSort(left, pivot, right, done)

    isDone := <-done

    if isDone {
        fmt.Println("\tDone sorting images!")
    } else {
        fmt.Println("\tisDone reported something fucky")
    }

    flagSort(db, req, resp)
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
    r.HandleFunc("/judge", JudgeHandler(db, imageComparisonRequests))
    r.HandleFunc("/vote", VoteHandler(db, imageComparisonResponses))
    r.HandleFunc("/shutdown", ShutdownHandler(srv, db))
    r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("static/"))))

    go flagSort(db, imageComparisonRequests, imageComparisonResponses)

    srv.ListenAndServe()
}
