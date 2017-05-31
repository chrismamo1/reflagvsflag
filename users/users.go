package users

import (
    "database/sql"
    _ "github.com/lib/pq")

type ID int

type User struct {
    Id ID
    Addr string
}

func GetByAddr(db *sql.DB, addr string) *User {
    u := new(User)
    err := db.QueryRow("SELECT id, ip_addr FROM users WHERE ip_addr = $1", addr).Scan(&u.Id, &u.Addr)
    if err != nil {
        return nil
    }
    return u
}
