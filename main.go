package main

import (
	"database/sql"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/chrismamo1/reflagvsflag/assets"
	sched "github.com/chrismamo1/reflagvsflag/comparisonScheduler"
	"github.com/chrismamo1/reflagvsflag/tags"
	"github.com/chrismamo1/reflagvsflag/things"
	"github.com/chrismamo1/reflagvsflag/users"
	"github.com/gorilla/mux"
	_ "github.com/lib/pq"
)

func initDb() *sql.DB {
	dbParams := os.ExpandEnv("user=db_master dbname=reflagvsflag_db sslmode=disable password=${REFLAGVSFLAG_DB_PASSWORD} host=${REFLAGVSFLAG_DB_HOST}")
	db, err := sql.Open("postgres", dbParams)
	if err != nil {
		log.Fatal(err)
	}

	statement := `
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

func loadImageStore(db *sql.DB, ts []string) []things.Thing {
	tx := things.GetTransactionWithTags(db, ts)

	rows, err := tx.Query("SELECT id, path, COALESCE(description, ''), img_index, heat, name, elo FROM imgs ORDER BY elo DESC")
	if err != nil {
		log.Fatal(err)
	}

	var imageStore []things.Thing

	for rows.Next() {
		var img things.Thing
		err = rows.Scan(&img.Id, &img.Path, &img.Desc, &img.Index, &img.Heat, &img.Name, &img.Elo)
		if err != nil {
			log.Fatal(err)
		}
		imageStore = append(imageStore, img)
	}
	rows.Close()
	tx.Commit()

	for i, img := range imageStore {
		imageStore[i].Tags = tags.GetTags(db, int(img.Id))
	}
	return imageStore
}

func addAllTagsCookie(db *sql.DB, w *http.ResponseWriter) {
	tags := tags.GetAllTags(db)
	sTags := make([]string, len(tags))
	for i, tag := range tags {
		sTags[i] = string(tag.Tag)
	}
	cookie := http.Cookie{Name: "all_tags", Value: strings.Join(sTags, ",")}
	http.SetCookie(*w, &cookie)
}

func addSelectedTagsCookie(selectedTags []string, w *http.ResponseWriter) {
	cookie := http.Cookie{
		Name:  "selected_tags",
		Value: strings.Join(selectedTags, ",")}
	http.SetCookie(*w, &cookie)
}

func WrapHandler(db *sql.DB, h func(http.ResponseWriter, *http.Request, []string, []string)) func(http.ResponseWriter, *http.Request) {
	return func(writer http.ResponseWriter, req *http.Request) {
		addAllTagsCookie(db, &writer)

		var selectedTagsCookie string
		var tagsCookie *http.Cookie

		tagsCookie, err := req.Cookie("selected_tags")
		if err != nil {
			selectedTagsCookie = "Modern"
		} else {
			selectedTagsCookie = tagsCookie.Value
		}

		userTags := strings.Split(selectedTagsCookie, ",")
		if len(userTags) < 1 {
			userTags = []string{"Modern"}
		}

		addSelectedTagsCookie(userTags, &writer)

		allTags := tags.GetAllTags(db)
		sAllTags := make([]string, len(allTags))
		for i, tag := range allTags {
			sAllTags[i] = string(tag.Tag)
		}

		h(writer, req, sAllTags, userTags)
	}
}

func VoteHandler(db *sql.DB, scheduler *sched.Scheduler) func(http.ResponseWriter, *http.Request, []string, []string) {
	return func(writer http.ResponseWriter, req *http.Request, allTags []string, tags []string) {
		redirect := func() {
			target := "/judge"

			writer.Header().Add("Location", target)
			writer.WriteHeader(302)
			page := `
            <h1>Thanks for voting!</h1>
            `
			writer.Write([]byte(page))
		}

		var ids things.IDPair
		winner, _ := strconv.Atoi(req.FormValue("winner"))
		loser, _ := strconv.Atoi(req.FormValue("loser"))

		log.Printf("voting: winner = %d, loser = %d\n", winner, loser)

		user := users.GetByAddr(db, req.RemoteAddr)
		user.SubmitVote(db, things.ID(winner), things.ID(loser))

		ids.Fst = things.ID(winner)
		ids.Snd = things.ID(loser)
		query := `
			SELECT "left", "right", balance, heat
			FROM comparisons
			WHERE
				(("left" = $1 AND "right" = $2) OR ("right" = $1 AND "left" = $2))
			`
		rows, err := db.Query(query, winner, loser)
		if err != nil {
			log.Println("Error selecting comparisons: ", err)
			redirect()
			return
		}
		defer rows.Close()

		nrows := 0

		var left, right, balance, heat int
		for rows.Next() {
			err = rows.Scan(&left, &right, &balance, &heat)
			if err != nil {
				log.Println(err)
				redirect()
				return
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
		if nrows == 0 {
			query = `
				INSERT INTO comparisons("left", "right", balance, heat)
				VALUES ($1, $2, -1, $3);`
			_, err = db.Exec(query, winner, loser, heat)
		} else {
			query = `
				UPDATE comparisons
				SET balance = $1, heat = $2
				WHERE "left" = $3 AND "right" = $4;`
			_, err = db.Exec(query, balance, heat, left, right)
		}
		if err != nil {
			log.Println(err)
			redirect()
			return
		}

		redirect()
		scheduler.FillRequest(things.ID(winner), things.ID(loser))
	}
}

func RanksHandler(db *sql.DB) func(http.ResponseWriter, *http.Request, []string, []string) {
	tmpl, err := template.ParseFiles("views/tags.gotemplate", "views/reflagvsflag.gotemplate", "views/ranks.gotemplate")
	if err != nil {
		log.Fatal("Error parsing the templates for RanksHandler: ", err)
	}

	type CParams struct {
		AllRanks []template.HTML
		TagSpecs []tags.UserTagSpec
	}

	return func(writer http.ResponseWriter, req *http.Request, allTags []string, uTags []string) {
		selTags := make([]tags.Tag, len(uTags))
		for i, tag := range uTags {
			selTags[i] = tags.Tag(tag)
		}

		tagSpecs := tags.MakeSpecs(db, selTags)

		users.GetByAddr(db, req.RemoteAddr)

		store := loadImageStore(db, uTags)

		els := []template.HTML{}

		for i := 0; i < len(store); i = i + 1 {
			els = append(els, things.RenderSmall(store[i]))
		}

		tmplParams := struct {
			ContentParams CParams
			Style         string
		}{ContentParams: CParams{
			AllRanks: els,
			TagSpecs: tagSpecs},
			Style: "ranks"}
		err := tmpl.ExecuteTemplate(writer, "container", tmplParams)
		if err != nil {
			log.Println("Error while executing template for ranks: ", err)
		}
	}
}

func StatsHandler(db *sql.DB) func(http.ResponseWriter, *http.Request, []string, []string) {
	tmpl, err := template.ParseFiles("views/reflagvsflag.gotemplate", "views/stats.gotemplate")
	if err != nil {
		log.Fatal("Error parsing the templates for StatsHandler: ", err)
	}

	type CParams struct {
		TotalVotes int
		TotalUsers int
		EloStdDev  float64
		HeatStdDev float64
		TotalFlags int
	}

	return func(writer http.ResponseWriter, req *http.Request, allTags []string, uTags []string) {
		addAllTagsCookie(db, &writer)

		var selectedTagsCookie string
		var tagsCookie *http.Cookie

		if tagsCookie, err = req.Cookie("selected_tags"); err != nil {
			selectedTagsCookie = "Modern"
		} else {
			selectedTagsCookie = tagsCookie.Value
		}

		userTags := strings.Split(selectedTagsCookie, ",")
		if len(userTags) < 1 {
			userTags = []string{"Modern"}
		}

		addSelectedTagsCookie(userTags, &writer)

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
			Style         string
		}{ContentParams: content,
			Style: "stats"}
		tmpl.ExecuteTemplate(writer, "container", tmplParams)
	}
}

func UsersHandler(db *sql.DB) func(http.ResponseWriter, *http.Request, []string, []string) {
	return func(writer http.ResponseWriter, req *http.Request, allTags []string, uTags []string) {
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

		page = fmt.Sprintf(page, els)

		writer.Write([]byte(page))
	}
}

func JudgeHandler(db *sql.DB, scheduler *sched.Scheduler) func(http.ResponseWriter, *http.Request, []string, []string) {
	tmpl, err := template.ParseFiles("views/tags.gotemplate", "views/reflagvsflag.gotemplate", "views/judge.gotemplate")
	if err != nil {
		log.Fatal("Error parsing the templates for JudgeHandler: ", err)
	}

	type CParams struct {
		FirstId  int
		First    template.HTML
		SecondId int
		Second   template.HTML
		TagSpecs []tags.UserTagSpec
	}

	return func(writer http.ResponseWriter, req *http.Request, allTags []string, uTags []string) {
		redirect := func() {
			writer.Header().Add("Location", "/judge")
			writer.WriteHeader(302)
			page := `
            <h1>Thanks for voting!</h1>
            `
			writer.Write([]byte(page))
		}

		tagSpecs := tags.GetAllTags(db)
		for i, t := range tagSpecs {
			for _, u := range uTags {
				if strings.Compare(string(t.Tag), u) == 0 {
					tagSpecs[i].Selected = true
					break
				}
			}
		}

		ids := scheduler.NextRequest(*users.GetByAddr(db, req.RemoteAddr), uTags)
		if ids == nil {
			redirect()
			return
		}

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

		left, right := things.SelectImages(db, *ids)
		tmplParams := struct {
			ContentParams CParams
			Style         string
		}{ContentParams: CParams{
			FirstId:  int(left.Id),
			First:    things.RenderNormal(left),
			SecondId: int(right.Id),
			Second:   things.RenderNormal(right),
			TagSpecs: tagSpecs},
			Style: "judge"}
		if err := tmpl.ExecuteTemplate(writer, "container", tmplParams); err != nil {
			log.Println("Failure executing JudgeHandler template: ", err)
		}
	}
}

func UploadHandler(db *sql.DB) func(http.ResponseWriter, *http.Request, []string, []string) {
	tmpl, err := template.ParseFiles("views/tags.gotemplate", "views/reflagvsflag.gotemplate", "views/upload.gotemplate")
	if err != nil {
		log.Fatal("Error parsing the templates for RanksHandler: ", err)
	}

	type CParams struct {
		TagSpecs []tags.UserTagSpec
	}

	return func(writer http.ResponseWriter, req *http.Request, allTags []string, uTags []string) {
		fail := func() {
			writer.Header().Add("Location", "/upload")
			writer.WriteHeader(302)
			page := `<h1>Invalid upload.</h1>`
			writer.Write([]byte(page))
		}

		if req.Method == "Get" {
			users.GetByAddr(db, req.RemoteAddr)

			tmplParams := struct {
				ContentParams CParams
				Style         string
			}{ContentParams: CParams{TagSpecs: tags.MakeSpecs(db, []tags.Tag{})},
				Style: "upload"}
			err := tmpl.ExecuteTemplate(writer, "container", tmplParams)
			if err != nil {
				log.Println("Error while executing template for upload: ", err)
			}
		} else {
			req.ParseMultipartForm(2 * (1 << 20)) // max memory of 2 megs
			file, header, err := req.FormFile("flag-path")
			imgName := assets.UploadImage(file, header)
			if err != nil {
				fail()
				return
			}
			rawTags := strings.Join(uTags, ",")
			flagName := req.FormValue("flag-name")
			flagPath := "http://d1tefi9crrjlgi.cloudfront.net/user-flags/" + imgName
			flagDesc := req.FormValue("flag-desc")
			log.Printf("Creating a flag with name \"%s\", path \"%s\", and tags %s\n", flagName, flagPath, rawTags)

			statement := `
                INSERT INTO images (path, name, img_index, heat, description)
                VALUES ($1, $2, -1, (SELECT AVG(heat) FROM images), $3)
            `
			if _, err := db.Exec(statement, flagPath, flagName, flagDesc); err != nil {
				log.Println("Error adding a flag: ", err)
				fail()
				return
			} else {
				var id int
				query := `SELECT id FROM images WHERE path = $1`
				if err := db.QueryRow(query, flagPath).Scan(&id); err != nil {
					log.Println("Error reading a flag after adding it: ", err)
					fail()
					return
				}
				for _, t := range uTags {
					statement := `
                        INSERT INTO image_tags (image, tag)
                        VALUES ($1, $2)`
					if _, err := db.Exec(statement, id, t); err != nil {
						log.Println("Error adding tags to a new flag: ", err)
						fail()
						return
					}
				}
				writer.Header().Add("Location", "/")
				writer.WriteHeader(302)
			}
		}
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

		_, err = tx.Exec(query, file.Name(), string(path), max+1)
		if err != nil {
			fmt.Printf("problem encountered while trying to run the query \"%s\":\n", query)
			fmt.Printf("(used values: \"%s\", \"%s\", %d)\n", file.Name(), string(path), max+1)
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
		Handler:      r,
		Addr:         ":80",
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}

	r.HandleFunc("/index", IndexHandler)
	r.HandleFunc("/index.html", IndexHandler)
	r.HandleFunc("/", IndexHandler)
	r.HandleFunc("/ranks", WrapHandler(db, RanksHandler(db)))
	r.HandleFunc("/users", WrapHandler(db, UsersHandler(db)))
	r.HandleFunc("/stats", WrapHandler(db, StatsHandler(db)))
	r.HandleFunc("/upload", WrapHandler(db, UploadHandler(db)))
	r.HandleFunc("/judge", WrapHandler(db, JudgeHandler(db, scheduler)))
	r.HandleFunc("/vote", WrapHandler(db, VoteHandler(db, scheduler)))
	r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("static/"))))

	fmt.Println("About to ListenAndServe")
	srv.ListenAndServe()
}
