package main

import (
	"errors"
	"html/template"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"
)

type Page struct {
	Title string
	Body  []byte
}

var (
	templates = template.Must(template.ParseFiles("templates/index.html", "templates/edit.html", "templates/view.html"))
	validPath = regexp.MustCompile("^/(edit|save|view)/([a-zA-Z0-9]+)$")
)

// Helper to load page files from the data directory, creating it if it doesn't exist
func getDataFileNames(path string) ([]string, error) {
	var fileNames []string

	// check that the directory exists, create it if not...
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		err := os.Mkdir(path, os.ModePerm)
		if err != nil {
			log.Printf("Directory %s doesn't exist and couldn't create\n", path)
			return fileNames, err
		}
	}

	// open the directory and get the files
	f, err := os.Open(path)
	if err != nil {
		log.Printf("Couldn't open directory %s: %s\n", path, err.Error())
		return fileNames, err
	}
	files, err := f.Readdir(-1)
	if err != nil {
		log.Printf("Couldn't read directory %s: %s\n", path, err.Error())
		return fileNames, err
	}

	for _, file := range files {
		name, _ := strings.CutSuffix(file.Name(), ".txt")
		fileNames = append(fileNames, name)
	}
	return fileNames, nil
}

// Page load and save functions
func (p *Page) save() error {
	filename := "data/" + p.Title + ".txt"
	return os.WriteFile(filename, p.Body, 0600)
}

func loadPage(title string) (*Page, error) {
	filename := "data/" + title + ".txt"
	body, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	return &Page{Title: title, Body: body}, nil
}

// Template helpers
func renderTemplate(w http.ResponseWriter, tmpl string, p *Page) {
	if err := templates.ExecuteTemplate(w, tmpl+".html", p); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// The HttpHandler funcs
func viewHandler(w http.ResponseWriter, r *http.Request, title string) {
	p, err := loadPage(title)
	if err != nil {
		http.Redirect(w, r, "/edit/"+title, http.StatusFound)
		return
	}
	renderTemplate(w, "view", p)
}

func editHandler(w http.ResponseWriter, r *http.Request, title string) {
	p, err := loadPage(title)
	if err != nil {
		p = &Page{Title: title}
	}
	renderTemplate(w, "edit", p)
}

func saveHandler(w http.ResponseWriter, r *http.Request, title string) {
	body := r.FormValue("body")
	p := &Page{Title: title, Body: []byte(body)}
	err := p.save()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/view/"+title, http.StatusFound)
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	files, err := getDataFileNames("data")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err = templates.ExecuteTemplate(w, "index.html", files); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// logging middleware
func logRequestHandler(h http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		// call the original handler we're wrapping
		h.ServeHTTP(w, r)

		// gather information about the request and log it
		uri := r.URL.String()
		method := r.Method

		log.Printf("%s:%s", uri, method)
	}
	return http.HandlerFunc(fn)
}

// HttpHandler wrapper to ensure valid paths are being passed into our handlers
func makeHandler(fn func(http.ResponseWriter, *http.Request, string)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		m := validPath.FindStringSubmatch(r.URL.Path)
		if m == nil {
			http.NotFound(w, r)
			return
		}
		fn(w, r, m[2])
	}
}

// Where all the magic happens...
func main() {
	mux := &http.ServeMux{}

	mux.HandleFunc("/", indexHandler)
	mux.HandleFunc("/view/", makeHandler(viewHandler))
	mux.HandleFunc("/edit/", makeHandler(editHandler))
	mux.HandleFunc("/save/", makeHandler(saveHandler))

	var handler http.Handler = mux
	handler = logRequestHandler(handler)
	srv := &http.Server{
		ReadTimeout:  120 * time.Second,
		WriteTimeout: 120 * time.Second,
		IdleTimeout:  120 * time.Second,
		Handler:      handler,
		Addr:         ":8080",
	}
	log.Fatal(srv.ListenAndServe())
}
