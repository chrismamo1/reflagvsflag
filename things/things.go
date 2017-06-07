package things

import (
    "bytes"
    "database/sql"
    "errors"
    "fmt"
    _ "github.com/lib/pq"
    "github.com/chrismamo1/reflagvsflag/tags"
    "html/template"
    "io/ioutil"
    "log"
    "regexp"
    "strings")

type ID int

type IDPair struct {
    Fst ID
    Snd ID
}

func (this *IDPair) Equivalent(x IDPair) bool {
    return (this.Fst == x.Fst && this.Snd == x.Snd) || (this.Fst == x.Snd && this.Snd == x.Fst)
}

type Thing struct {
    Id ID
    Name string
    Path string
    Desc string
    Index int
    Heat int
    Elo float64
    Tags []tags.Tag
}

type Comparison struct {
    Left int
    Right int
    Balance int
}

func render(thing Thing, root string, maxWidth int, maxHeight int, showElo bool) template.HTML {
    matched, err := regexp.MatchString(".*\\.url$", thing.Path)
    if err != nil {
        log.Fatal(err)
    }
    var format string
    if matched {
        url, err := ioutil.ReadFile("." + root + thing.Path)
        if err != nil {
            log.Fatal(err)
        }
        thing.Path = string(url)
        matched, err := regexp.MatchString(".*\\.gifv$", thing.Path)
        if err != nil {
            log.Fatal(err)
        }
        if matched {
            format = `
            <video
            style='max-width: {{.MaxWidth}}px; max-height: {{.MaxHeight}}px; box-shadow: 0px 0px 5px black'>
            <source src='{{.Path}}' type='video/mp4'>
            </video>
            `
        } else {
            if showElo {
                format = `
                    <div style="padding: 5px">
                        <center>
                            <h3>{{.Name}} (ELO: {{.Elo}})</h3>
                        </center>
                        <figure>
                            <img
                                style='width: 100%; max-height: 100%; box-shadow: 0px 0px 5px black'
                                src='{{.Path}}'>
                            </img>
                            <figcaption>{{.Desc}}</figcaption>
                        </figure>
                    </div>
                `
            } else {
                format = `
                    <div style="padding: 5px">
                        <center>
                            <h3>{{.Name}}</h3>
                        </center>
                        <figure>
                            <img
                                style='width: 100%; max-height: 100%; box-shadow: 0px 0px 5px black'
                                src='{{.Path}}'>
                            </img>
                            <figcaption>{{.Desc}}</figcaption>
                        </figure>
                    </div>
                `
            }
        }
    } else {
        if showElo {
            format = `
                <div style="padding: 5px">
                    <center>
                        <h3>{{.Name}} (ELO: {{.Elo}})</h3>
                    </center>
                    <figure>
                        <img
                            style='width: 100%; max-height: 100%; box-shadow: 0px 0px 5px black'
                            src='{{.Path}}'>
                        </img>
                        <figcaption>{{.Desc}}</figcaption>
                    </figure>
                </div>
            `
        } else {
            format = `
                <div style="padding: 5px">
                    <center>
                        <h3>{{.Name}}</h3>
                    </center>
                    <figure>
                        <img
                            style='width: 100%; max-height: 100%; box-shadow: 0px 0px 5px black'
                            src='{{.Path}}'>
                        </img>
                        <figcaption>{{.Desc}}</figcaption>
                    </figure>
                </div>
            `
        }
        thing.Path = root + thing.Path
    }
    templ, err := template.New("image").Parse(format)
    if err != nil {
        log.Fatal(err)
    }

    type Parameters struct {
        MaxWidth int
        MaxHeight int
        Path string
        Name string
        Elo float64
        Desc string
    }

    var params Parameters
    params.MaxWidth = maxWidth
    params.MaxHeight = maxHeight
    params.Path = strings.Trim(thing.Path, "\n\r")
    params.Name = thing.Name
    params.Elo = thing.Elo
    params.Desc = thing.Desc

    buffer := bytes.NewBufferString("")

    err = templ.Execute(buffer, params)
    if err != nil {
        log.Fatal(err)
    }
    return template.HTML(buffer.String())
}

func RenderSmall(thing Thing) template.HTML {
    return render(thing, "", 200, 200, true)
}

func RenderNormal(thing Thing) template.HTML {
    return render(thing, "", 600, 600, false)
}

func getHead2HeadComparison(db *sql.DB, a ID, b ID) (Comparison, error) {
    query := "SELECT \"left\", \"right\", balance FROM comparisons WHERE ((\"left\" = %d AND \"right\" = %d) OR (\"right\" = %d AND \"left\" = %d))"
    query = fmt.Sprintf(query, a, b, a, b)
    rows, err := db.Query(query)
    if err != nil {
        log.Fatal(err)
    }
    defer rows.Close()

    var left,right,balance int
    var cmp Comparison
    for rows.Next() {
        err = rows.Scan(&left, &right, &balance)
        if err != nil {
            log.Fatal(err)
        }
        if (left == int(a) && right == int(b)) || (left == int(b) && right == int(a)) {
            cmp.Left = left
            cmp.Right = right
            cmp.Balance = balance
            return cmp, nil
        }
    }
    return cmp, errors.New("no matching found between the specified images")
}

func getAllNeighbouringComparisons(db *sql.DB, a ID, b ID) ([]Comparison, error) {
    query := `
    SELECT "left", "right", balance FROM comparisons
    WHERE NOT(("left" = %d AND "right" = %d) OR ("right" = %d AND "left" = %d)) -- exclude direct comparisons between a and b
    AND

    )
    `
    query = fmt.Sprintf(query, a, b, a, b, a, b, a, b)
    rows, err := db.Query(query)
    if err != nil {
        log.Fatal(err)
    }
    defer rows.Close()

    var left,right,balance int
    var cmps []Comparison
    for rows.Next() {
        err = rows.Scan(&left, &right, &balance)
        if err != nil {
            log.Fatal(err)
        }
        var cmp Comparison
        cmp.Left = left
        cmp.Right = right
        cmp.Balance = balance
        cmps = append(cmps, cmp)
    }
    return cmps, nil
}

func GetComparison(db *sql.DB, a ID, b ID) int {
    res, err := getHead2HeadComparison(db, a, b)
    if err != nil {
        //log.Fatal(err)
        return 0
    } else {
        if res.Left == int(a) {
            return res.Balance
        }
        if res.Left == int(b) {
            return -res.Balance
        }
        return 0;
    }
}

func SelectImages(db *sql.DB, ids IDPair) (Thing, Thing) {
    tx, err := db.Begin()
    if err != nil {
        log.Fatal("Error beginning the transaction to get tags in SelectImages: ", err)
    }

    q := `
    SELECT id, path, COALESCE(description, ''), img_index, heat, COALESCE(name, ''), elo
    FROM images
    WHERE id = $1 OR id = $2;
    `
    fmt.Printf("selecting images %d and %d\n", int(ids.Fst), int(ids.Snd))
    rows, err := tx.Query(q, ids.Fst, ids.Snd)
    if err != nil {
        fmt.Print("syntax error in selectImages query: ")
        log.Fatal(err)
    }

    var img1,img2 Thing
    rows.Next();
    err = rows.Scan(&img1.Id, &img1.Path, &img1.Desc, &img1.Index, &img1.Heat, &img1.Name, &img1.Elo)
    if err != nil {
        fmt.Println("A\n")
        log.Fatal(err)
    }
    rows.Next();
    err = rows.Scan(&img2.Id, &img2.Path, &img2.Desc, &img2.Index, &img2.Heat, &img2.Name, &img2.Elo)
    if err != nil {
        fmt.Println("B\n")
        log.Fatal(err)
    }

    rows.Close();

    img1.Heat = img1.Heat + 1
    img2.Heat = img2.Heat + 1

    query := fmt.Sprintf("UPDATE images SET heat = %d WHERE id = %d;", img1.Heat, img1.Id)
    query = fmt.Sprintf("%s; UPDATE images SET heat = %d WHERE id = %d;", query, img2.Heat, img2.Id)
    _, err = tx.Exec(query);
    if err != nil {
        log.Fatal(err)
    }

    tx.Commit()

    img1.Tags = tags.GetTags(db, int(img1.Id))
    img2.Tags = tags.GetTags(db, int(img2.Id))

    return img1,img2
}
/* please note that this function is nondeterministic: it only returns a random element from the set
   of elements which have the minimum heat */
func GetColdestPair(db *sql.DB) IDPair {
    var ids IDPair
    query := `
    SELECT "left", "right"
    FROM comparisons
    WHERE heat = (SELECT heat FROM comparisons ORDER BY heat ASC LIMIT 1)
    ORDER BY RANDOM()
    LIMIT 1
    `
    err := db.QueryRow(query).Scan(&ids.Fst, &ids.Snd)
    if err != nil {
        // the comparisons table might be empty, let's try getting two random ID's
        err := db.QueryRow("SELECT id FROM images ORDER BY RANDOM() LIMIT 1").Scan(&ids.Fst)
        if err != nil {
            log.Fatal(err)
        }
        err = db.QueryRow("SELECT id FROM images WHERE id != $1 ORDER BY RANDOM() LIMIT 1", ids.Fst).Scan(&ids.Snd)
        if err != nil {
            log.Fatal(err)
        }
    }
    fmt.Printf("coldest pair: %d, %d\n", ids.Fst, ids.Snd)
    return ids
}

func GetRandomPair(db *sql.DB) IDPair {
    var ids IDPair
    err := db.QueryRow("SELECT id FROM images ORDER BY RANDOM() LIMIT 1").Scan(&ids.Fst)
    if err != nil {
        log.Fatal(err)
    }
    err = db.QueryRow("SELECT id FROM images WHERE id != $1 ORDER BY RANDOM() LIMIT 1", ids.Fst).Scan(&ids.Snd)
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("random pair: %d, %d\n", ids.Fst, ids.Snd)
    return ids
}

func GetRandomIdAboveIndex(db *sql.DB, index int) ID {
    var rv ID
    err := db.QueryRow("SELECT id FROM images WHERE img_index > $1 ORDER BY RANDOM() LIMIT 1", index).Scan(&rv)
    if err != nil {
        log.Fatal(err)
    }
    return rv
}

func GetRandomPairAboveIndex(db *sql.DB, index int) IDPair {
    var ids IDPair
    ids.Fst = GetRandomIdAboveIndex(db, index)
    err := db.QueryRow("SELECT id FROM images WHERE id != $1 AND img_index > $2 ORDER BY RANDOM() LIMIT 1", ids.Fst, index).Scan(&ids.Snd)
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("random pair: %d, %d\n", ids.Fst, ids.Snd)
    return ids
}

func GetTags(db *sql.DB) []string {
    var tags []string
    query := `SELECT name FROM tags`
    rows, err := db.Query(query)
    if err != nil {
        log.Fatal("Error while trying to query tags: ", err)
    }
    for rows.Next() {
        var tag string
        rows.Scan(&tag)
        tags = append(tags, tag)
    }
    return tags
}

func GetTransactionWithTags(db *sql.DB, tags []string) *sql.Tx {
    var tx *sql.Tx
    tx, err := db.Begin()
    if err != nil {
        log.Fatal("Error starting a transaction in GetTransactionWithTags: ", err)
    }

    statement := `
        CREATE TEMPORARY TABLE imgs (
            id INTEGER,
            path TEXT NOT NULL UNIQUE,
            name TEXT,
            description TEXT,
            img_index INT,
            heat INT NOT NULL,
            elo REAL NOT NULL DEFAULT(1000.0)
        ) ON COMMIT DROP;
        CREATE TEMPORARY TABLE given_tags (
            tag TEXT UNIQUE
        ) ON COMMIT DROP;
    `
    if _, err := tx.Exec(statement); err != nil {
        log.Fatal("Error making temp tables in GetTransactionWithTags: ", err)
    }

    for _, t := range(tags) {
        statement := `INSERT INTO given_tags (tag) VALUES ($1)`
        if _, err := tx.Exec(statement, t); err != nil {
            log.Fatal("Error adding a tag to given_tags in GetTransactionWithTags: ", err)
        }
    }

    statement = `
        INSERT INTO imgs (id, path, name, description, img_index, heat, elo)
        SELECT DISTINCT i.id, i.path, i.name, i.description, i.img_index, i.heat, i.elo
        FROM (
            SELECT *
            FROM images
            LEFT OUTER JOIN image_tags ON images.id = image_tags.image
            WHERE tag IN (SELECT tag FROM given_tags)) i
    `
    if _, err := tx.Exec(statement); err != nil {
        log.Fatal("Error populating imgs in GetTransactionWithTags: ", err)
    }

    for _, t := range(tags) {
        var id int
        log.Printf("Trying with tag %s\n", t)
        query := `SELECT id FROM imgs`
        if err := tx.QueryRow(query).Scan(&id); err != nil {
            log.Println("Couldn't get anything out of imgs: ", err)
        }
        log.Printf("IMGS has a thing %d for tag %s\n", id, t)
    }

    return tx
}
