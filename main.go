package main

import (
    "database/sql"
    "fmt"
    "io/ioutil"
    "log"
    "net/http"
    "os"
    //"runtime"
    "strconv"
    "html/template"
    "strings"
    "time"
    "github.com/gorilla/mux"
    _ "github.com/lib/pq"
    sched "github.com/chrismamo1/reflagvsflag/comparisonScheduler"
    "github.com/chrismamo1/reflagvsflag/things"
    "github.com/chrismamo1/reflagvsflag/users")

func initDb() *sql.DB {
    dbParams := os.ExpandEnv("user=db_master dbname=reflagvsflag_db sslmode=disable password=${REFLAGVSFLAG_DB_PASSWORD} host=${REFLAGVSFLAG_DB_HOST}")
    db, err := sql.Open("postgres", dbParams)
    if err != nil {
        log.Fatal(err)
    }

    /*statement := `
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
    }*/

    statement := `
    CREATE TABLE IF NOT EXISTS images (
        id SERIAL PRIMARY KEY,
        path TEXT NOT NULL UNIQUE,
        name TEXT,
        description TEXT,
        img_index INT NOT NULL,
        heat INT NOT NULL,
        elo REAL NOT NULL DEFAULT(1000.0));
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
    CREATE TABLE IF NOT EXISTS tags (
        name TEXT NOT NULL);
    INSERT INTO sort_iterations (count) VALUES (0) ON CONFLICT DO NOTHING;
    TRUNCATE scheduler;
    UPDATE exposure SET heat = 0;
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
    rows, err := db.Query("SELECT id, path, description, img_index, heat, name, elo FROM images ORDER BY elo DESC")
    if err != nil {
        log.Fatal(err)
    }
    defer rows.Close()

    var imageStore []things.Thing

    for rows.Next() {
        var img things.Thing
        err = rows.Scan(&img.Id, &img.Path, &img.Desc, &img.Index, &img.Heat, &img.Name, &img.Elo)
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

        log.Printf("voting: winner = %d, loser = %d\n", winner, loser)

        user := users.GetByAddr(db, req.RemoteAddr)
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
        scheduler.FillRequest(things.ID(winner), things.ID(loser))
    }
}

func RanksHandler(db *sql.DB) func(http.ResponseWriter, *http.Request) {
    tmpl, err := template.ParseFiles("views/reflagvsflag.html", "views/ranks.html")
    if err != nil {
        log.Fatal("Error parsing the templates for RanksHandler: ", err)
    }

    type CParams struct {
        AllRanks []template.HTML
    }

    return func(writer http.ResponseWriter, req *http.Request) {
        users.GetByAddr(db, req.RemoteAddr)

        store := loadImageStore(db)

        els := []template.HTML{}

        for i := 0; i < len(store); i = i + 1 {
            els = append(els, things.RenderSmall(store[i]))
        }

        tmplParams := struct {
            ContentParams CParams
            Style string
        } { ContentParams: CParams{ AllRanks: els },
            Style: "ranks" }
        tmpl.ExecuteTemplate(writer, "container", tmplParams)
    }
}

func StatsHandler(db *sql.DB) func(http.ResponseWriter, *http.Request) {
    tmpl, err := template.ParseFiles("views/reflagvsflag.html", "views/stats.html")
    if err != nil {
        log.Fatal("Error parsing the templates for StatsHandler: ", err)
    }

    type CParams struct {
        TotalVotes int
        TotalUsers int
        EloStdDev float64
        HeatStdDev float64
        TotalFlags int
    }

    return func(writer http.ResponseWriter, req *http.Request) {
        var content CParams
        users.GetByAddr(db, req.RemoteAddr)

        query := `SELECT COUNT(*) FROM votes;`
        if err := db.QueryRow(query).Scan(&content.TotalVotes); err != nil {
            log.Fatal("Error trying to get the total number of votes: ", err)
        }

        query = `SELECT COUNT(*) FROM users;`
        if err := db.QueryRow(query).Scan(&content.TotalUsers); err != nil {
            log.Fatal("Error trying to get the total number of users: ", err)
        }

        query = `SELECT STDDEV(elo) FROM images;`
        if err := db.QueryRow(query).Scan(&content.EloStdDev); err != nil {
            log.Fatal("Error trying to get the standard deviation for Elo's: ", err)
        }

        query = `SELECT COUNT(*) FROM images;`
        if err := db.QueryRow(query).Scan(&content.TotalFlags); err != nil {
            log.Fatal("Error trying to get the total number of flags: ", err)
        }

        query = `SELECT STDDEV(heat) FROM images;`
        if err := db.QueryRow(query).Scan(&content.HeatStdDev); err != nil {
            log.Fatal("Error trying to get the stddev of heat for all images: ", err)
        }

        tmplParams := struct {
            ContentParams CParams
            Style string
        } { ContentParams: content,
            Style: "stats" }
        tmpl.ExecuteTemplate(writer, "container", tmplParams)
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
    tmpl, err := template.ParseFiles("views/reflagvsflag.html", "views/judge.html")
    if err != nil {
        log.Fatal("Error parsing the templates for JudgeHandler: ", err)
    }

    type CParams struct {
        FirstId int
        First template.HTML
        SecondId int
        Second template.HTML
        Tags []string
    }

    return func(writer http.ResponseWriter, req *http.Request) {
        tags := strings.Split(req.FormValue("tags"), ",")

        ids := scheduler.NextRequest(*users.GetByAddr(db, req.RemoteAddr), tags)

        bumpExposure := func(user *users.User, img things.ID) {
            var exists bool
            query := `SELECT (EXISTS(SELECT * FROM views WHERE "user" = $1 AND image = $2))`
            err := db.QueryRow(query, user.Id, img).Scan(&exists)
            if err != nil {
                log.Fatal(err)
            }
            if exists {
                query := `UPDATE views SET heat = heat + 1 WHERE "user" = $1 AND image = $2`
                if _, err := db.Exec(query, user.Id, img); err != nil {
                    log.Fatal(err)
                }
            } else {
                query := `INSERT INTO views ("user", image, heat) VALUES ($1, $2, 1)`
                if _, err := db.Exec(query, user.Id, img); err != nil {
                    log.Fatal(err)
                }
            }
        }

        user := users.GetByAddr(db, req.RemoteAddr)

        bumpExposure(user, ids.Fst)
        bumpExposure(user, ids.Snd)

        left, right := things.SelectImages(db, ids)
        tmplParams := struct {
            ContentParams CParams
            Style string
        } { ContentParams: CParams{
                FirstId: int(left.Id),
                First: things.RenderNormal(left),
                SecondId: int(right.Id),
                Second: things.RenderNormal(right),
                Tags: things.GetTags(db) },
            Style: "judge" }
        tmpl.ExecuteTemplate(writer, "container", tmplParams)
        /*page = fmt.Sprintf(page, left.Id, right.Id, things.RenderNormal(left), right.Id, left.Id, things.RenderNormal(right))
        writer.Write([]byte(page))*/
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

func main() {
    fmt.Println("About to initialize the database")
    db := initDb()
    defer fmt.Println("Closing shit")
    defer db.Close()

    scheduler := sched.Make(db, 1)

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
    r.HandleFunc("/stats", StatsHandler(db))
    r.HandleFunc("/judge", JudgeHandler(db, scheduler))
    r.HandleFunc("/vote", VoteHandler(db, scheduler))
    r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("static/"))))

    fmt.Println("About to ListenAndServe")
    srv.ListenAndServe()
}
