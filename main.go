package main

import (
	"database/sql"
	"fmt"
	"html/template"
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
	log.Printf(os.ExpandEnv("DB Host: ${REFLAGVSFLAG_DB_HOST}\n"))
	db, err := sql.Open("postgres", dbParams)
	if err != nil {
		log.Fatal("Problem opening a connection to the SQL server: ", err)
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

		var selectedTagsCookie string = ""
		var tagsCookie *http.Cookie

		tagsCookie, err := req.Cookie("selected_tags")
		if req.FormValue("tags") != "" {
			selectedTagsCookie = req.FormValue("tags")
		}
		if err != nil {
			selectedTagsCookie = selectedTagsCookie + "," + "Modern"
		} else {
			selectedTagsCookie = selectedTagsCookie + "," + tagsCookie.Value
		}

		tmpTags := strings.Split(selectedTagsCookie, ",")
		if len(tmpTags) < 1 || (len(tmpTags) == 1 && tmpTags[0] == "") {
			tmpTags = []string{"Modern"}
		}

		allTags := tags.GetAllTags(db)

		var userTags []string

		for _, t := range tmpTags {
			for _, r := range allTags {
				if t == string(r.Tag) {
					already := false
					for _, u := range userTags {
						if u == t {
							already = true
							break
						}
					}
					if !already {
						tmp := append(userTags, t)
						userTags = tmp
					}
				}
			}
		}

		addSelectedTagsCookie(userTags, &writer)

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

		winner, _ := strconv.Atoi(req.FormValue("winner"))
		loser, _ := strconv.Atoi(req.FormValue("loser"))

		if strings.Compare(req.FormValue("isTie"), "yes") == 0 {
			scheduler.RemoveRequest(things.ID(winner), things.ID(loser))
			redirect()
			return
		}

		log.Printf("voting: winner = %d, loser = %d\n", winner, loser)

		user := users.GetByAddr(db, req.RemoteAddr)
		user.SubmitVote(db, things.ID(winner), things.ID(loser))

		redirect()
		scheduler.FillRequest(tags, things.ID(winner), things.ID(loser))
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
			query := `UPDATE images SET heat = heat + 1 WHERE id = $1`
			_, err := db.Exec(query, img)
			if err != nil {
				log.Println("Error updating heat for an image: ", err)
			}
			/*var exists bool
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
			}*/
		}

		user := users.GetByAddr(db, req.RemoteAddr)

		bumpExposure(user, ids.Fst)
		bumpExposure(user, ids.Snd)

		left, right := things.SelectImages(db, *ids)
		left.Name = ""
		right.Name = ""
		left.Desc = ""
		right.Desc = ""
		tmplParams := struct {
			ContentParams CParams
			Style         string
		}{ContentParams: CParams{
			FirstId:  int(left.Id),
			First:    things.RenderNormal(left, false),
			SecondId: int(right.Id),
			Second:   things.RenderNormal(right, false),
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

		if req.Method == "GET" {
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
			user := users.GetByAddr(db, req.RemoteAddr)
			log.Println("About to try to create a flag...")
			req.ParseMultipartForm(2 * (1 << 20)) // max memory of 2 megs
			file, header, err := req.FormFile("flag-path")
			if err != nil {
				fail()
				return
			}
			imgName := assets.UploadImage(file, header)
			err = file.Close()
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
                INSERT INTO images (path, name, img_index, heat, description, uploaded_by)
                VALUES ($1, $2, -1, (SELECT MIN(heat) FROM images), $3, $4)
            `
			if _, err := db.Exec(statement, flagPath, flagName, flagDesc, user.Id); err != nil {
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

func main() {
	fmt.Println("About to initialize the database")
	db := initDb()
	defer fmt.Println("Closing shit")
	defer db.Close()

	scheduler := sched.Make(db, 1)

	fmt.Println("About to create the mux")
	r := mux.NewRouter()

	srv := &http.Server{
		Handler:      r,
		Addr:         "0.0.0.0:80",
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
