package users

import (
    "fmt"
    "time"
    //"bytes"
    "database/sql"
    "github.com/chrismamo1/reflagvsflag/things"
    "log"
    "strings"
    //"html/template"
    _ "github.com/lib/pq")

type ID int

type User struct {
    Id ID
    Addr string
}

type vote struct {
    id int
    user ID
    winner things.ID
    loser things.ID
    submitted_at time.Time
}

func New(db *sql.DB, addr string) User {
    _, err := db.Exec("INSERT INTO users (ip_addr) VALUES ($1)", addr)
    if err != nil {
        log.Fatal(err)
    }

    var u User

    err = db.QueryRow("SELECT id, ip_addr FROM users WHERE ip_addr = $1", addr).Scan(&u.Id, &u.Addr)
    if err != nil {
        log.Fatal("Failed to get a user we just created: ", err)
    }

    rows, err := db.Query(`SELECT "id" FROM images`)
    for rows.Next() {
        var id things.ID

        err := rows.Scan(&id)
        if err != nil {
            log.Fatal("Failed to scan an image's ID while creating a new user: ", err)
        }

        db.Exec(`INSERT INTO exposure ("user", image, heat) VALUES ($1, $2, 0)`, u.Id, id)
        if err != nil {
            log.Fatal("Failed to add an exposure entry while creating a new user: ", err)
        }
    }

    return u
}

func GetByAddr(db *sql.DB, addr string) *User {
    if strings.Index(addr, ":") != -1 {
        addr = addr[0:strings.Index(addr,":")]
    }
    u := new(User)
    err := db.QueryRow("SELECT id, ip_addr FROM users WHERE ip_addr = $1", addr).Scan(&u.Id, &u.Addr)
    if err != nil {
        // user doesn't exist yet, we can remedy this
        fmt.Println("creating a new user because we failed at trying to get it...\n")
        *u = New(db, addr)
    }
    return u
}

func (this *User) GetVotes(db *sql.DB) []vote {
    rows, err := db.Query("SELECT id, \"user\", winner, loser, submitted_at FROM votes WHERE \"user\" = $1", this.Id)
    if err != nil {
        log.Fatal(err)
    }

    var votes []vote

    for rows.Next() {
        var v vote
        if err := rows.Scan(&v.id, &v.user, &v.winner, &v.loser, &v.submitted_at); err != nil {
            log.Fatal("problem scanning a user's vote: ", err)
        }
        votes = append(votes, v)
    }

    return votes
}

func renderVotes(votes []vote) string {
    buffer := "<div><table><tbody>"
    buffer += `
        <tr>
            <td>ID</td>
            <td>winner</td>
            <td>loser</td>
            <td>Submission Time</td>
        </tr>
    `

    for _, v := range(votes) {
        format := `
            <tr>
                <td>%d</td>
                <td>%d</td>
                <td>%d</td>
                <td>%s</td>
            </tr>
        `
        el := fmt.Sprintf(format, v.id, v.winner, v.loser, v.submitted_at.String())
        buffer += el
    }

    buffer += "</tbody></table></div>"
    return buffer
}

/// TODO: finish this
func (this *User) Render(db *sql.DB, className string) string {
    /*format := `
        <div class='{{.ClassName}}'>
            <h1>ID: {{.Id}}</h1>
            <h2>IP Address: {{.Addr}}</h2>
            <div>
                <h1>Votes:</h1>
                {{.RenderedVotes}}
            </div>
        </div>
    `
    templ, err := template.New("user").Parse(format)
    if err != nil {
        log.Fatal(err)
    }

    renderedVotes := renderVotes(this.GetVotes(db))

    params := struct {
        ClassName string
        Addr string
        RenderedVotes string
        Id ID
    } {ClassName: className, Addr: this.Addr, Id: this.Id, RenderedVotes: renderedVotes}

    buffer := bytes.NewBufferString("")

    err = templ.Execute(buffer, params)
    if err != nil {
        log.Fatal(err)
    }

    return buffer.String()*/

    renderedVotes := renderVotes(this.GetVotes(db))

    return `
        <div class='` + className + `'>
            <h1>ID: {{.Id}}</h1>
            <h2>IP Address: ` + this.Addr + `</h2>
            <div>
                <h1>Votes:</h1>
                ` + renderedVotes + `
            </div>
        </div>
        `
}

func GetAll(db *sql.DB) []User {
    rows, err := db.Query("SELECT id, ip_addr FROM users")
    if err != nil {
        log.Fatal(err)
    }

    var users []User

    for rows.Next() {
        var u User
        if err := rows.Scan(&u.Id, &u.Addr); err != nil {
            log.Fatal("problem scanning a user's vote: ", err)
        }
        users = append(users, u)
    }

    return users
}

func (this *User) SubmitVote(db *sql.DB, winner things.ID, loser things.ID) {
    query := `INSERT INTO votes ("user", winner, loser) VALUES ($1, $2, $3)`
    _, err := db.Exec(query, this.Id, winner, loser)
    if err != nil {
        log.Fatal(err)
    }

    /// TODO: move bumpExposure into this function

    /// TODO: move the code to adjust comparisons into this function

    return
}
