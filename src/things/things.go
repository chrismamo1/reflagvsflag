package things

import (
        "html/template"
        "log"
        "bytes"
        "database/sql"
        "fmt"
        "errors"
        "regexp"
        _ "github.com/mattn/go-sqlite3"
        "io/ioutil")

type ID int

type IDPair struct {
        Fst ID
        Snd ID
}

type Thing struct {
        Id ID
        Path string
        Desc string
        Index int
        Heat int
}

type Comparison struct {
        Left int
        Right int
        Balance int
}

func render(thing Thing, root string, maxWidth int, maxHeight int) string {
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
                        format = "<img style='max-width: {{.MaxWidth}}px; max-height: {{.MaxHeight}}px; box-shadow: 0px 0px 5px black' src='{{.Path}}'></img>"
                }
        } else {
                format = "<img style='max-width: {{.MaxWidth}}px; max-height: {{.MaxHeight}}px; box-shadow: 0px 0px 5px black' src='{{.Path}}'></img>"
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
        }

        var params Parameters
        params.MaxWidth = maxWidth
        params.MaxHeight = maxHeight
        params.Path = thing.Path

        buffer := bytes.NewBufferString("")

        err = templ.Execute(buffer, params)
        if err != nil {
                log.Fatal(err)
        }
        return buffer.String()
}

func RenderSmall(thing Thing) string {
        return render(thing, "/static/img/", 200, 200)
}

func RenderNormal(thing Thing) string {
        return render(thing, "/static/img/", 600, 600)
}

func getHead2HeadComparison(db *sql.DB, a ID, b ID) (Comparison, error) {
        query := "SELECT left, right, balance FROM comparisons WHERE ((left = %d AND right = %d) OR (right = %d AND left = %d))"
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
        SELECT left, right, balance FROM comparisons
        WHERE NOT((left = %d AND right = %d) OR (right = %d AND left = %d)) -- exclude direct comparisons between a and b
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
                        if res.Balance < 0 {
                                return -1
                        } else if res.Balance > 0 {
                                return 1
                        }
                }
                if res.Left == int(b) {
                        if res.Balance < 0 {
                                return 1
                        } else if res.Balance > 0 {
                                return -1
                        }
                }
                return 0;
        }
}
