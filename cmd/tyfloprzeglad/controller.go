package main

import (
	"html/template"
	"io/ioutil"
	"net/http"
	"strconv"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/markbates/pkger"

	"github.com/mikolysz/tyfloprzeglad"
)

type Controller struct {
	repo                tyfloprzeglad.Repo
	r                   *chi.Mux
	list, details, edit *view
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
		details: newView("episode_details", "form"),
		edit:    newView("edit", "form"),
	}

	r.Get("/", c.listEpisodes)
	r.Post("/", c.createEpisode)
	r.Get("/{slug}", c.viewEpisodeDetails)
	r.Post("/{slug}", c.addStory)
	r.Get("/{slug}/{segment_id}/{story_id}/edit", c.editStory)
	r.Post("/{slug}/{segment_id}/{story_id}/edit", c.updateStory)
	r.Post("/{slug}/{segment_id}/{story_id}/delete", c.deleteStory)

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
		"episode": e,
		"storyForm": storyForm{
			Story:      &tyfloprzeglad.Story{},
			Presenters: c.repo.PresenterNames(),
			Segments:   e.SegmentNames(),
		},
	}
	must(c.details.Execute(w, data))
}

func (c *Controller) addStory(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")

	s, segment, err := parseStoryForm(r)
	must(err)

	must(c.repo.AddStory(slug, segment, s))

	// Users complained that duplicate stories were added when refreshing the page.
	// If you refresh after sending a post request, the request gets send again and the story gets duplicated.
	// The browser warns you about this, but users don't read what the browser says.
	// For that reason, we can't just display the updated page, but we need to do a redirect instead.
	// A redirect is going to cause  a GET request , and that's what's going to be repeated after a refresh.
	http.Redirect(w, r, "/"+slug, http.StatusSeeOther)
}

func (c *Controller) editStory(w http.ResponseWriter, r *http.Request) {
	s, err := c.fetchStoryForEditing(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	f := storyForm{
		Editing:    true,
		Story:      s,
		Presenters: c.repo.PresenterNames(),
	}

	c.edit.Execute(w, f)
}

func (c *Controller) updateStory(w http.ResponseWriter, r *http.Request) {
	s, err := c.fetchStoryForEditing(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	// We currently can't change a story's segment, so we just discard that information.
	new, _, err := parseStoryForm(r)
	if err != nil {
		http.NotFound(w, r)
	}

	s.Title = new.Title
	s.Notes = new.Notes
	s.Presenter = new.Presenter

	c.repo.Save()
	http.Redirect(w, r, "/"+chi.URLParam(r, "slug"), http.StatusSeeOther)
}

func (c *Controller) fetchStoryForEditing(r *http.Request) (s *tyfloprzeglad.Story, err error) {
	// Parse request params
	var (
		slug      = chi.URLParam(r, "slug")
		segmentID = chi.URLParam(r, "segment_id")
		storyID   = chi.URLParam(r, "story_id")
	)

	segmentIdI, err := strconv.Atoi(segmentID)
	if err != nil {
		return nil, err
	}

	storyIdI, err := strconv.Atoi(storyID)
	if err != nil {
		return nil, err
	}

	// Retrieve the story to be edited
	e, err := c.repo.EpisodeBySlug(slug)
	if err != nil {
		return nil, err
	}

	if len(e.Segments) < segmentIdI {
		return nil, err
	}
	seg := e.Segments[segmentIdI]

	s, err = seg.StoryByID(storyIdI)
	if err != nil {
		return nil, err
	}

	return s, nil
}

func (c *Controller) deleteStory(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	segmentID, err := strconv.Atoi(chi.URLParam(r, "segment_id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}

	storyID, err := strconv.Atoi(chi.URLParam(r, "story_id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}

	if err := c.repo.DeleteStory(slug, segmentID, storyID); err != nil {
		http.NotFound(w, r)
		return
	}

	http.Redirect(w, r, "/"+slug, http.StatusSeeOther)
}

func (c *Controller) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	c.r.ServeHTTP(w, r)
}

type view struct {
	*template.Template
}

var defaultTemplates = []string{"layout"}

func newView(tmplNames ...string) *view {
	t := template.New("main")
	templates := append(defaultTemplates[:], tmplNames...)

	for _, n := range templates {
		path := "/templates/" + n + ".tmpl"
		f, err := pkger.Open(path)
		if err != nil {
			panic(err)
		}

		b, err := ioutil.ReadAll(f)
		if err != nil {
			panic(err)
		}

		str := string(b)

		template.Must(t.Parse(str))
	}
	return &view{t}
}

// must is a helper function that panics if the passed error is not nil, does nothing otherwise.
func must(err error) {
	if err != nil {
		panic(err.Error())
	}
}
