package main

import (
	"net/http"

	"github.com/mikolysz/tyfloprzeglad"
)

// storyForm contains the data necessary to display the form for adding and editing stories.
type storyForm struct {
	Editing    bool                 // Are we editing an existing story or creating a new one?
	Story      *tyfloprzeglad.Story // Contains values for existing fields, Story{} for a new story.
	Presenters []string             // The list of possible presenters that we can choose from.

	// Only relevant for new stories:
	Segments []string // Segments in this episode.
}

func parseStoryForm(r *http.Request) (s *tyfloprzeglad.Story, segment string, err error) {
	if err = r.ParseForm(); err != nil {
		return nil, "", err
	}

	f := r.PostForm
	s = &tyfloprzeglad.Story{
		Title:     f.Get("title"),
		Notes:     f.Get("notes"),
		Presenter: f.Get("presenter"),
	}

	return s, f.Get("segment"), nil
}
