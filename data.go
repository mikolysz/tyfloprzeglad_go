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
	Slug     string
	Segments []*Segment
}

type Segment struct {
	Name    string
	Stories []*Story
}

type Story struct {
	Title     string
	Notes     string
	Presenter string
}

type Repo struct {
	Episodes        []*Episode
	DefaultSegments []string
	Presenters      []string
}

func (r *Repo) AddEpisode(title string) (*Episode, error) {
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

func (r *Repo) EpisodeList() ([]*Episode, error) {
	return r.Episodes, nil
}

func (r *Repo) EpisodeBySlug(slug string) (*Episode, error) {
	for _, e := range r.Episodes {
		if e.Slug == slug {
			return e, nil
		}
	}
	return nil, ErrorNotFound
}

func (r *Repo) AddStory(slug string, segment string, story *Story) error {
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

func (r *Repo) Save(filename string) error {
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	d := json.NewEncoder(f)
	d.SetIndent("", "	")
	return d.Encode(r)
}

func RepoFromFile(filename string) (*Repo, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	d := json.NewDecoder(f)
	r := &Repo{}
	if err := d.Decode(r); err != nil {
		return nil, err
	}
	return r, nil
}
