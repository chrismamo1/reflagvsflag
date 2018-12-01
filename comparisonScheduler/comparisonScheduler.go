package comparisonScheduler

import (
	"database/sql"
	"errors"
	"log"
	"math"
	"math/rand"
	"time"

	"github.com/chrismamo1/reflagvsflag/things"
	"github.com/chrismamo1/reflagvsflag/users"
	_ "github.com/lib/pq"
)

type Priority int

const (
	PMarginal Priority = iota
	PLow
	PMedium
	PHigh
)

type Scheduler struct {
	db                 *sql.DB
	pointlessThreshold int
}

func (this *Scheduler) hasAnyRequests() bool {
	var status bool
	query := `SELECT EXISTS(SELECT * FROM schedule LIMIT 1)`
	if err := this.db.QueryRow(query).Scan(&status); err != nil {
		log.Fatal("Error while trying to see if the scheduler table has anything in it: ", err)
	}
	return status
}

func (this *Scheduler) getMinPlacement() int {
	placement := 1
	if this.hasAnyRequests() {
		query := `select MIN(placement) FROM scheduler`
		if err := this.db.QueryRow(query).Scan(&placement); err != nil {
			log.Fatal("Error while getting the max placement from the scheduler while appending a request: ", err)
		}
	}
	return placement
}

func (this *Scheduler) hasRequest(ids things.IDPair) bool {
	var status bool
	query := `SELECT EXISTS(SELECT * FROM schedule WHERE (fst = $1 AND snd = $2) OR (fst = $2 AND snd = $1) LIMIT 1)`
	if err := this.db.QueryRow(query, ids.Fst, ids.Snd).Scan(&status); err != nil {
		log.Fatal("Error while trying to determine if the scheduler contains a request: ", err)
	}
	return status
}

func (this *Scheduler) rmRequest(ids things.IDPair) {
	statement := `DELETE FROM schedule WHERE (fst = $1 AND snd = $2) OR (fst = $2 AND snd = $1)`
	if _, err := this.db.Exec(statement, ids.Fst, ids.Snd); err != nil {
		log.Fatal("Error while trying to delete a scheduler request: ", err)
	}
}

func (this *Scheduler) HasRequest(ids things.IDPair) bool {
	return this.hasRequest(ids)
}

func (this *Scheduler) FillRequest(tags []string, winner things.ID, loser things.ID) {
	ids := things.IDPair{Fst: winner, Snd: loser}
	log.Printf("Filling a request for %d, %d\n", int(winner), int(loser))
	if this.hasRequest(ids) {
		var elo1, elo2 float64
		var aCount, otherCount int
		tx := things.GetTransactionWithTags(this.db, tags)
		defer tx.Commit()
		if err := tx.QueryRow(`SELECT COUNT(*) FROM imgs`).Scan(&aCount); err != nil {
			log.Fatal("Error getting the count of tags that the user is being presented with", err)
		}
		if err := this.db.QueryRow(`SELECT COUNT(*) FROM images`).Scan(&otherCount); err != nil {
			log.Fatal("Error getting the total count of all images", err)
		}
		participation := float64(aCount) / float64(otherCount)
		participation = participation * participation
		query := `SELECT elo FROM images WHERE id = $1;`
		if err := this.db.QueryRow(query, winner).Scan(&elo1); err != nil {
			log.Fatal("Error getting 1st elo in FillRequest: ", err)
		}
		if err := this.db.QueryRow(query, loser).Scan(&elo2); err != nil {
			log.Fatal("Error getting 2nd elo in FillRequest: ", err)
		}
		r1 := math.Pow(2.0, elo1/400)
		r2 := math.Pow(2.0, elo2/400)
		e1 := r1 / (r1 + r2)
		e2 := r2 / (r1 + r2)
		s1 := 1.0
		s2 := 0.0
		elo1 = elo1 + 10.0*(s1-e1)*participation
		elo2 = elo2 + 10.0*(s2-e2)*participation
		log.Println("Participation: ", participation)
		log.Printf("New ELO for image %d: %f\n", int(winner), elo1)
		log.Printf("New ELO for image %d: %f\n", int(loser), elo2)
		statement := "UPDATE images SET elo = $1 WHERE id = $2"
		if _, err := this.db.Exec(statement, elo1, winner); err != nil {
			log.Fatal("Error while trying to update an ELO value: ", err)
		}
		statement = "UPDATE images SET elo = $1 WHERE id = $2"
		if _, err := this.db.Exec(statement, elo2, loser); err != nil {
			log.Fatal("Error while trying to update an ELO value: ", err)
		}
		this.rmRequest(ids)
		if this.hasRequest(ids) {
			log.Fatal(errors.New("rmRequest doesn't work"))
		}
	}
}

func (this *Scheduler) RemoveRequest(fst things.ID, snd things.ID) {
	var ids things.IDPair
	ids.Fst = fst
	ids.Snd = snd
	this.rmRequest(ids)
}

func (this *Scheduler) NextRequest(user users.User, tags []string) *things.IDPair {
	if len(tags) < 1 {
		tags = []string{"Modern"}
	}
	tx := things.GetTransactionWithTags(this.db, tags)

	rand.Seed(time.Now().UnixNano())

	var ids things.IDPair
	var s_heat int
	var elo float64

	query := `
        SELECT id, heat, elo
        FROM
            imgs
        ORDER BY heat + RANDOM() * 2 ASC LIMIT 1
    `
	if err := tx.QueryRow(query).Scan(&ids.Fst, &s_heat, &elo); err != nil {
		log.Println("Error keeping us from employing the user-based heat check: ", err)
		if err := tx.Commit(); err != nil {
			log.Fatal("Error while trying to commit the aborted transaction: ", err)
		}

		tx = things.GetTransactionWithTags(this.db, tags)
		query := `
            SELECT imgs.id
            FROM imgs
            ORDER BY imgs.heat ASC, RANDOM()
            LIMIT 2
        `
		rows, err := tx.Query(query)
		if err != nil {
			log.Fatal("Error selecting totally random elements in NextRequest: ", err)
			rows.Close()
			tx.Commit()
			return nil
		}
		if rows.Next() != true {
			log.Println("Error getting the first random element in NextRequest (rows.Next() returned false)")
			rows.Close()
			tx.Commit()
			return nil
		}
		if err := rows.Scan(&ids.Fst); err != nil {
			log.Println("Error while scanning the first ID in NextRequest: ", err)
			rows.Close()
			tx.Commit()
			return nil
		}
		if rows.Next() != true {
			log.Println("Error getting the first random element in NextRequest (rows.Next() returned false)")
			rows.Close()
			tx.Commit()
			return nil
		}
		if err := rows.Scan(&ids.Snd); err != nil {
			log.Println("Error while scanning the second ID in NextRequest: ", err)
			rows.Close()
			tx.Commit()
			return nil
		}
		rows.Close()
	} else {
		query := `
            SELECT id
            FROM
                (SELECT *
                 FROM imgs
                 ORDER BY ABS(elo-$1) ASC
                 LIMIT 10) tbl
            WHERE id <> $2
            ORDER BY heat + RANDOM() * 2 LIMIT 1;
        `
		if err := tx.QueryRow(query, elo, ids.Fst).Scan(&ids.Snd); err != nil {
			log.Println("Error selecting: ", err)
			tx.Commit()
			return nil
		}
	}

	defer tx.Commit()

	statement := `INSERT INTO schedule (fst, snd, "user") VALUES ($1, $2, $3)`
	if _, err := tx.Exec(statement, ids.Fst, ids.Snd, user.Id); err != nil {
		log.Fatal("Error trying to add an element to the schedule: ", err)
	}

	return &ids
}

func Make(db *sql.DB, pointlessAt int) *Scheduler {
	return &Scheduler{db: db, pointlessThreshold: pointlessAt}
}
