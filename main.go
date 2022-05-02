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

// Page describes how we present an article
type Page struct {
	Title string
	Body []byte
	PageID int
	Links []string
}

// Configuration for setting up database
type Configuration struct {
	Dbuser string `json:"dbuser"`
	Dbname string `json:"dbname"`
	Dbpass string `json:"dbpass"`
	Dbaddr string `json:"dbaddr"`
}

var templates = template.Must(template.ParseFiles("edit.html", "view.html", "search.html"))
var validPath = regexp.MustCompile("^/(edit|save|view|search)/([a-zA-Z0-9]+)$")

func (p *Page) save() (error) {
	/*
	Saves a Page by inserting it into database

	Args:
		p (*Page): page to save
	*/
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

func search(text string) ([]Page, error) {
	/*
	Searches for a keyword in the database

	Args:
		text (string): takes a text to search in the page body and title
	*/
	search_text := "%" + text + "%"
	var list_pages []Page
	rows, err := db.Query(`SELECT * FROM pages WHERE title LIKE ? OR body LIKE ?`, search_text, search_text)
	if err != nil {
		return  nil, fmt.Errorf("Error: name: %q: %v", search_text, err)
	}
	defer rows.Close()
	// loop through rows
	for rows.Next() {
		var p Page
		if err := rows.Scan(&p.PageID, &p.Title, &p.Body); err != nil {
			return nil, fmt.Errorf("Error: name: %q: %v", search_text, err)
		}
		list_pages = append(list_pages, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("Error: %q: %v", search_text, err)
	}
	return list_pages, nil
}


func loadPage(title string) (*Page, error) {
	/*
	Loads a page

	Args:
		title (string): takes in a title for requesting the page
	*/
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
	/*
	Renders the html template

	Args:
		w (http.ResponseWriter): interface used by HTTP to construct response
		templateName (string): name of the html template
		p (*Page): Takes in a page
	*/
	err := templates.ExecuteTemplate(w, templateName + ".html", p)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}


func makeHandler(fn func (http.ResponseWriter, *http.Request, string)) http.HandlerFunc {
	/*
	Creates a handler

	Args:
		fn (func): takes in a handler function
	*/
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
	/*
	A handler for viewing pages, and managing nonexistent pages

	Args:
		w (http.ResponseWriter): interface used by HTTP to construct response
		r (*http.Request): HTTP request received by server or sent by client
		title (string): takes in a title for requesting the page
	*/
	p, err := loadPage(title)
	if err != nil {
		p = &Page{Title:title}
		http.Redirect(w, r, "/edit/" + p.Title, http.StatusFound)
		return
	}

	renderTemplate(w, "view", p)
}

func saveHandler(w http.ResponseWriter, r *http.Request, title string) {
	/*
	A handler for saving pages
	
	Args:
		w (http.ResponseWriter): interface used by HTTP to construct response
		r (*http.Request): HTTP request received by server or sent by client
		title (string): takes in a title for requesting the page
	*/
	body := r.FormValue("body")
	p := &Page{Title: title, Body: []byte(body)}
	err := p.save()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	http.Redirect(w, r, "/view/"+title, http.StatusFound)
}

func editHandler(w http.ResponseWriter, r *http.Request, title string) {
	/*
	A handler for editting pages

	Args:
		w (http.ResponseWriter): interface used by HTTP to construct response
		r (*http.Request): HTTP request received by server or sent by client
		title (string): takes in a title for requesting the page
	*/
	p, err := loadPage(title)
	if err != nil {
		p = &Page{Title: title}
	}
	renderTemplate(w, "edit", p)
}

func searchHandler(w http.ResponseWriter, r *http.Request, title string) {
	/*
	A handler for saving pages

	Args:
		w (http.ResponseWriter): interface used by HTTP to construct response
		r (*http.Request): HTTP request received by server or sent by client
		title (string): takes in a title for requesting the page
	*/
	search_text := r.FormValue("body")
	results, err := search(search_text)
	if err != nil {
		fmt.Printf("No results found")
	}
	var list_title []string
	var list_body [][]byte
	for _, pages := range results {
		list_title = append(list_title, pages.Title)
		list_body = append(list_body, pages.Body)
	}
	p := &Page{Title: "search", Links: list_title}
	renderTemplate(w, "search", p)
}

func main() {
	/*
	main function for running the gowiki application
	*/
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
	http.HandleFunc("/search/",makeHandler(searchHandler))
	log.Fatal(http.ListenAndServe(":3000", nil))
}
