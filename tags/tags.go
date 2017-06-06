package tags

import (
    "database/sql"
    _ "github.com/lib/pq"
    "log")

type Tag string

type UserTagSpec struct {
    Tag Tag
    Selected bool
}

func GetTags(db *sql.DB, thing int /* should be a things.ID */) []Tag {
    rows, err := db.Query(`SELECT tag FROM image_tags WHERE image = $1`, thing)
    defer rows.Close()
    if err != nil {
        log.Fatal("Error selecting rows from image_tags in GetTags: ", err)
    }

    var rval []Tag

    for rows.Next() {
        var t Tag
        if err := rows.Scan(&t); err != nil {
            log.Fatal("Error scanning a tag in GetTags: ", err)
        }
        rval = append(rval, t)
    }

    return rval
}

func GetAllTags(db *sql.DB) []UserTagSpec {
    rows, err := db.Query(`SELECT name FROM tags`)
    defer rows.Close()
    if err != nil {
        log.Fatal("Error selecting rows from tags in GetAllTags: ", err)
    }

    var rval []UserTagSpec

    for rows.Next() {
        var t UserTagSpec
        if err := rows.Scan(&t.Tag); err != nil {
            log.Fatal("Error scanning a tag in GetAllTags: ", err)
        }
        t.Selected = false
        rval = append(rval, t)
    }

    return rval
}
