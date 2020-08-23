package main

import (
	"encoding/json"
	"errors"
	"os"
	"strings"
	"unicode"
)

var ErrorNotFound = errors.New("not found")

type Episode struct {
	Title    string
	Slug     string // The title with weird characters stripped, so it can i.e. be embedded into an url.
	Segments []*Segment
}

// Segment contains stories with a particular theme for a given episode.
// When an episode is created, a list of empty segments is populated.
type Segment struct {
	Name    string
	Stories []*Story
}

type Story struct {
	Title     string
	Notes     string
	Presenter string
}

// Repo stores the data needed for the app to function.
type Repo interface {
	// AddEpisode adds an episode with the given title.
	AddEpisode(title string) (*Episode, error)
	// EpisodeList returns a list of all episodes, newest first.
	EpisodeList() ([]*Episode, error)
	// EpisodeBySlug retrieves an episode with a given slug.
	// If no such episode is found, NotFoundError is returned.
	EpisodeBySlug(slug string) (*Episode, error)
	// AddStory adds a story to a specific segment of an episode.
	AddStory(episodeSlug string, segment string, s *Story) error
	// Presenters returns the list of presenters available.
	PresenterNames() []string
}

func NewRepo(filename string) (Repo, error) {
	r := &repo{}
	return newJSONRepo(filename, r)
}

// jsonRepo is a repo that serializes and deserializes itself from a JSON file.
// Every time a change is made, a new JSON file is saved.
type jsonRepo struct {
	filename string
	Repo
}

func newJSONRepo(filename string, next Repo) (Repo, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	d := json.NewDecoder(f)
	if err := d.Decode(next); err != nil {
		return nil, err
	}
	return &jsonRepo{filename, next}, nil
}

func (r *jsonRepo) AddEpisode(title string) (*Episode, error) {
	e, err := r.Repo.AddEpisode(title)
	if err != nil {
		return nil, err
	}

	if err := r.Save(); err != nil {
		return nil, err
	}
	return e, nil
}

func (r *jsonRepo) AddStory(episodeSlug, segment string, s *Story) error {
	if err := r.Repo.AddStory(episodeSlug, segment, s); err != nil {
		return err
	}
	if err := r.Save(); err != nil {
		return err
	}
	return nil
}

func (r *jsonRepo) Save() error {
	f, err := os.Create(r.filename)
	if err != nil {
		return err
	}
	defer f.Close()

	d := json.NewEncoder(f)
	d.SetIndent("", "	")
	return d.Encode(r.Repo)
}

// repo simply stores data in memory, without any serialization or validation.
// Such concerns are handled by outer layers.
type repo struct {
	Episodes        []*Episode
	DefaultSegments []string
	Presenters      []string
}

func (r *repo) AddEpisode(title string) (*Episode, error) {
	e := &Episode{
		Title: title,
		Slug:  slugFromTitle(title),
	}

	for _, s := range r.DefaultSegments {
		e.Segments = append(e.Segments, &Segment{Name: s})
	}

	r.Episodes = append(r.Episodes, e)
	return e, nil
}

func slugFromTitle(title string) string {
	title = strings.ToLower(title)
	return strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			return r
		}
		return '-'
	}, title)
}

func (r *repo) EpisodeList() ([]*Episode, error) {
	return r.Episodes, nil
}

func (r *repo) EpisodeBySlug(slug string) (*Episode, error) {
	for _, e := range r.Episodes {
		if e.Slug == slug {
			return e, nil
		}
	}
	return nil, ErrorNotFound
}

func (r *repo) AddStory(slug string, segment string, story *Story) error {
	e, err := r.EpisodeBySlug(slug)
	if err != nil {
		return err
	}
	s, err := e.SegmentByName(segment)
	if err != nil {
		return err
	}
	s.Stories = append(s.Stories, story)
	return nil
}

func (e *Episode) SegmentByName(segment string) (*Segment, error) {
	for _, s := range e.Segments {
		if s.Name == segment {
			return s, nil
		}
	}
	return nil, ErrorNotFound
}

func (r *repo) PresenterNames() []string {
	return r.Presenters
}
