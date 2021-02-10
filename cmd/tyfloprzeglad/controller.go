package main

import (
	"html/template"
	"io/ioutil"
	"net/http"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/markbates/pkger"

	"github.com/mikolysz/tyfloprzeglad"
)

type Controller struct {
	repo          tyfloprzeglad.Repo
	r             *chi.Mux
	list, details *view
}

func NewController(repo tyfloprzeglad.Repo, user, pass string) *Controller {
	r := chi.NewRouter()

	// If   an error occurs during a request, we panic, so we need a middleware that recovers from  such errors and tells users that something went wrong.
	r.Use(middleware.Recoverer)

	users := map[string]string{user: pass}
	r.Use(middleware.BasicAuth("Authorization Required", users))

	// We must manually tell pkger to embed the "templates" directory directly in the binary.
	// We don't pass  string literals to pkger.Open, so the code-analyzing tool  can't figure this out on its own.
	pkger.Include("/templates/")

	c := &Controller{
		repo:    repo,
		r:       r,
		list:    newView("index"),
		details: newView("episode_details"),
	}

	r.Get("/", c.listEpisodes)
	r.Post("/", c.createEpisode)
	r.Get("/{slug}", c.viewEpisodeDetails)
	r.Post("/{slug}", c.addStory)
	return c
}

func (c *Controller) listEpisodes(w http.ResponseWriter, h *http.Request) {
	eps, err := c.repo.EpisodeList()
	must(err)
	must(c.list.Execute(w, eps))
}

func (c *Controller) createEpisode(w http.ResponseWriter, r *http.Request) {
	must(r.ParseForm())
	t := r.PostForm.Get("title")
	e, err := c.repo.AddEpisode(t)
	must(err)

	http.Redirect(w, r, "/"+e.Slug, http.StatusFound)
}

func (c *Controller) viewEpisodeDetails(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	e, err := c.repo.EpisodeBySlug(slug)
	if err == tyfloprzeglad.ErrorNotFound {
		http.NotFound(w, r)
		return
	}
	must(err)
	data := map[string]interface{}{
		"episode":    e,
		"presenters": c.repo.PresenterNames(),
	}
	must(c.details.Execute(w, data))
}

func (c *Controller) addStory(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	must(r.ParseForm())
	f := r.PostForm
	s := &tyfloprzeglad.Story{
		Title:     f.Get("title"),
		Notes:     f.Get("notes"),
		Presenter: f.Get("presenter"),
	}
	must(c.repo.AddStory(slug, f.Get("segment"), s))

	// Users complained that duplicate stories were added when refreshing the page.
	// If you refresh after sending a post request, the request gets send again and the story gets duplicated.
	// The browser warns you about this, but users don't read what the browser says.
	// For that reason, we can't just display the updated page, but we need to do a redirect instead.
	// A redirect is going to cause  a GET request , and that's what's going to be repeated after a refresh.
	http.Redirect(w, r, "/"+slug, http.StatusSeeOther)
}

func (c *Controller) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	c.r.ServeHTTP(w, r)
}

type view struct {
	*template.Template
}

func newView(tmplName string) *view {
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
	return &view{
		Template: template.Must(t.Parse(str)),
	}
}

// must is a helper function that panics if the passed error is not nil, does nothing otherwise.
func must(err error) {
	if err != nil {
		panic(err)
	}
}
