package users

import (
    "fmt"
    "time"
    "bytes"
    "database/sql"
    "github.com/chrismamo1/reflagvsflag/things"
    "log"
    "html/template"
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

func GetByAddr(db *sql.DB, addr string) *User {
    u := new(User)
    err := db.QueryRow("SELECT id, ip_addr FROM users WHERE ip_addr = $1", addr).Scan(&u.Id, &u.Addr)
    if err != nil {
        return nil
    }
    return u
}

func (this *User) GetVotes(db *sql.DB) []vote {
    rows, err := db.Query("SELECT id, \"user\", winner, loser, submitted_at FROM votes WHERE \"user\" = $1", this.Id)
    if err != nil {
        log.Fatal(err)
    }

    votes := make([]vote, 32)

    for rows.Next() {
        var v vote
        if err := rows.Scan(&v.id, &v.user, &v.winner, &v.loser); err != nil {
            log.Fatal("problem scanning a user's vote: ", err)
        }
        votes = append(votes, v)
    }

    return votes
}

func renderVotes(votes []vote) string {
    buffer := "<table>"
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

    buffer += "</table>"
    return buffer
}

/// TODO: finish this
func (this *User) Render(db *sql.DB, className string) string {
    format := `
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

    return buffer.String()
}

func GetAll(db *sql.DB) []User {
    rows, err := db.Query("SELECT id, ip_addr FROM users")
    if err != nil {
        log.Fatal(err)
    }

    users := make([]User, 32)

    for rows.Next() {
        var u User
        if err := rows.Scan(&u.Id, &u.Addr); err != nil {
            log.Fatal("problem scanning a user's vote: ", err)
        }
        users = append(users, u)
    }

    return users
}
