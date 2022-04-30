package main

import (
	"io/ioutil"
	"encoding/json"
	"net/http"
	"html/template"
	"regexp"
	"log"
	"fmt"
	"database/sql"
	"github.com/go-sql-driver/mysql"
	"os"
)

var db *sql.DB

//Page describes how we present an article
type Page struct {
	Title string
	Body []byte
	PageID int
}


type Configuration struct {
	Dbuser string `json:"dbuser"`
	Dbname string `json:"dbname"`
	Dbpass string `json:"dbpass"`
	Dbaddr string `json:"dbaddr"`
}

var templates = template.Must(template.ParseFiles("edit.html", "view.html"))
var validPath = regexp.MustCompile("^/(edit|save|view)/([a-zA-Z0-9]+)$")

func (p *Page) save() (error) {
	row := db.QueryRow("SELECT * FROM pages WHERE title = ?", p.Title)
	var err error
	n := *p
	if row.Scan(&n.PageID, &n.Title, &n.Body) == sql.ErrNoRows {
		_, err = db.Exec("INSERT INTO pages (title, body) VALUES (?, ?)", p.Title, p.Body)
	} else {
		_, err = db.Exec("UPDATE pages SET body=? WHERE title=?", p.Body, p.Title)
	}
	return err
}

func loadPage(title string) (*Page, error) {
	var p Page

	row := db.QueryRow("SELECT * FROM pages WHERE title = ?", title)
	if err := row.Scan(&p.PageID, &p.Title, &p.Body); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("%s: No page exists", title)
		}
		return nil , fmt.Errorf("Page ID: %d", p.PageID)
	}
	return &Page{Title: title, Body: p.Body}, nil
}

func renderTemplate(w http.ResponseWriter, templateName string, p *Page) {
	err := templates.ExecuteTemplate(w, templateName + ".html", p)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func makeHandler(fn func (http.ResponseWriter, *http.Request, string)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		m := validPath.FindStringSubmatch(r.URL.Path)
		if m == nil {
			http.NotFound(w, r)
			return
		}
		fn(w, r, m[2])
	}
}

func viewHandler(w http.ResponseWriter, r *http.Request, title string) {
	p, err := loadPage(title)
	if err != nil {
		p = &Page{Title:title}
		http.Redirect(w, r, "/edit/" + p.Title, http.StatusFound)
		return
	}

	renderTemplate(w, "view", p)
}

func saveHandler(w http.ResponseWriter, r *http.Request, title string) {
	body := r.FormValue("body")
	p := &Page{Title: title, Body: []byte(body)}
	err := p.save()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	http.Redirect(w, r, "/view/"+title, http.StatusFound)
}

func editHandler(w http.ResponseWriter, r *http.Request, title string) {
	p, err := loadPage(title)
	if err != nil {
		p = &Page{Title: title}
	}
	renderTemplate(w, "edit", p)
}

func main() {

	config_file, _ := os.Open("config.json")
	defer config_file.Close()
	config_data, err := ioutil.ReadAll(config_file)
	if err != nil {
		fmt.Printf("Failed to read config.json, error: %s", err)
	}
	config := Configuration{}
	err = json.Unmarshal(config_data, &config)
	if err != nil {
		fmt.Printf("error: %s\n", err)
	}

	cfg := mysql.Config{
		User: config.Dbuser,
		Passwd: config.Dbpass,
		Net: "tcp",
		Addr: config.Dbaddr,
		DBName: config.Dbname,
		AllowNativePasswords: true,
	}
	db, err = sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		log.Fatal(err)
	}

	pingErr := db.Ping()
	if pingErr != nil {
		log.Fatal(pingErr)
	}
	fmt.Println("Connection Successful!")

	http.HandleFunc("/view/", makeHandler(viewHandler))
	http.HandleFunc("/edit/", makeHandler(editHandler))
	http.HandleFunc("/save/", makeHandler(saveHandler))
	log.Fatal(http.ListenAndServe(":3000", nil))
}
