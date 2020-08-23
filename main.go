package main

import (
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"os"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/markbates/pkger"
)

func main() {
	// Retrieve data from environment variables.
	port := os.Getenv("PORT")
	if port == "" {
		port = "4000"
	}

	user := os.Getenv("TYFLOPRZEGLAD_USER")
	if user == "" {
		user = "user"
	}

	pass := os.Getenv("TYFLOPRZEGLAD_PASS")
	if pass == "" {
		pass = "pass"
	}

	filename := "tyfloprzeglad.json"
	repo, err := RepoFromFile(filename)
	if err != nil {
		log.Fatalf("Error when opening data file: %s", err)
	}

	r := chi.NewRouter()
	r.Use(middleware.BasicAuth("Authorization Required", map[string]string{user: pass}))
	r.Use(middleware.Recoverer)

	pkger.Include("/templates/")

	idx := newView("index")
	details := newView("episode_details")

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		eps, err := repo.EpisodeList()
		must(err)
		must(idx.Execute(w, eps))
	})

	r.Post("/", func(w http.ResponseWriter, r *http.Request) {
		must(r.ParseForm())
		t := r.PostForm.Get("title")
		e, err := repo.AddEpisode(t)
		must(err)
		must(repo.Save(filename))
		http.Redirect(w, r, "/"+e.Slug, http.StatusFound)
	})

	viewDetails := func(w http.ResponseWriter, r *http.Request) {
		slug := chi.URLParam(r, "slug")
		e, err := repo.EpisodeBySlug(slug)
		if err == ErrorNotFound {
			http.NotFound(w, r)
			return
		}
		must(err)
		data := map[string]interface{}{
			"episode":    e,
			"presenters": repo.Presenters,
		}

		must(details.Execute(w, data))
	}
	r.Get("/{slug}", viewDetails)

	r.Post("/{slug}", func(w http.ResponseWriter, r *http.Request) {
		slug := chi.URLParam(r, "slug")
		must(r.ParseForm())
		f := r.PostForm
		s := &Story{
			Title:     f.Get("title"),
			Notes:     f.Get("notes"),
			Presenter: f.Get("presenter"),
		}
		must(repo.AddStory(slug, f.Get("segment"), s))
		must(repo.Save(filename))
		viewDetails(w, r)
	})

	log.Println("Running on port ", port)
	http.ListenAndServe(":"+port, r)
}

type view struct {
	*template.Template
}

func newView(tmplName string) view {
	path := "/templates/" + tmplName + ".tmpl"
	f, err := pkger.Open(path)
	if err != nil {
		panic(err)
	}

	b, err := ioutil.ReadAll(f)
	if err != nil {
		panic(err)
	}

	str := string(b)

	t := template.New(tmplName)
	return view{
		Template: template.Must(t.Parse(str)),
	}
}

// must is a helper function that panics if the passed error is not nil, does nothing otherwise.
func must(err error) {
	if err != nil {
		panic(err)
	}
}
