package tyfloprzeglad

import (
	"encoding/json"
	"errors"
	"html/template"
	"math/rand"
	"os"
	"strings"
	"unicode"

	"mvdan.cc/xurls/v2"
)

var ErrorNotFound = errors.New("not found")

// urlRegexp is the regular expression used to parse URLs.
// It is safe for concurrent use.
var urlRegexp = xurls.Relaxed()

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
	ID        int
	Title     string
	Notes     string
	Presenter string
}

// NotesHTML returns the contents of the notes field as safe, escaped HTML.
// Newlines are converted to proper html <br>s, and URLs are turned into clickable, html links.
func (s *Story) NotesHTML() template.HTML {
	notes := template.HTMLEscapeString(s.Notes)
	notes = strings.Replace(notes, "\n", "<br>", -1)
	return template.HTML(turnURLsIntoLinks(notes))
}

func turnURLsIntoLinks(text string) string {
	str := ""
	lastIdx := 0
	idxs := urlRegexp.FindAllStringIndex(text, -1)

	for _, match := range idxs {
		beg, end := match[0], match[1]
		url := text[beg:end]

		str += text[lastIdx:beg] +
			"<a href=\"" +
			normalizeURL(url) +
			"\">" +
			url +
			"</a>"

		lastIdx = end
	}
	str += text[lastIdx:]
	return str
}

// normalizeURL adds http:// to bare URLs like google.com
func normalizeURL(url string) string {
	if strings.Contains(url, "://") {
		return url
	}
	return "http://" + url
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

	// migrate migrates the underlying database to the latest version if needed.
	migrate() error
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

	r := &jsonRepo{filename, next}
	if err := r.migrate(); err != nil {
		return nil, err
	}

	return r, nil
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
	DBVersion       int
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

	episodes := make([]*Episode, 0, len(r.Episodes))
	episodes = append(episodes, e)
	episodes = append(episodes, r.Episodes...)
	r.Episodes = episodes
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

	// Story IDs are assigned sequentially, starting at 1.
	// Therefore, if we have n stories, the highest assigned ID is n.
	story.ID = len(s.Stories) + 1
	s.addStory(story)
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

func (s *Segment) addStory(story *Story) {
	// When adding stories, we try to spread stories for one presenter throughout a segment, regardless how many  of them they add at a time.
	// This is done so that one person doesn't talk for too long, but presenters alternate.
	// To do this, we find a list of  indexes in the "Stories" slice where our story can go, and then select one at random, inserting the story there.
	//
	// First, we need to find situations where someone other than the presenter of our story has two stories immediately after one another.
	// This is something we don't want and need to fix urgently.
	// We usually don't let this happen, but sometimes we must, usually when one presenter has a lot of stories or they're the first to add some to a given segment.
	// If we notice something like that, we insert our story inbetween the two to rectify the situation.
	indexes := s.consecutiveStories(story.Presenter)
	if len(indexes) > 0 {
		s.insertStory(story, indexes)
		return
	}

	// If no such situations exist, we try to find places appropriate for our story.
	// An appropriate place is one where the stories before and after ours (if there are any)will be presented by someone else.
	// This ensures we don't have any two consecutive stories by one presenter.
	indexes = s.findAppropriatePlaces(story.Presenter)
	if len(indexes) > 0 {
		s.insertStory(story, indexes)
		return
	}

	// If we have so many stories that there's no appropriate place for the next one, let's just stick it at the end of the list.
	// We hope that other presenters add more stories at some point, which will split ours up.
	s.Stories = append(s.Stories, story)
}

// consecutiveStories finds situations where someone else than the given presenter has two consecutive stories.
// The returned indexes point to the second stories from the pairs, so inserting a story there will split them up.
func (s *Segment) consecutiveStories(presenter string) []int {
	indexes := make([]int, 0)
	for i := 0; i < len(s.Stories)-1; i++ {
		// If we find a story by the presenter we were given, we skip it .
		// After all, inserting a story here is something we're so desperately trying to avoid.
		if s.Stories[i].Presenter == presenter {
			continue
		}

		if s.Stories[i].Presenter == s.Stories[i+1].Presenter {
			// Houston, we have a problem...
			indexes = append(indexes, i+1)
		}
	}
	return indexes
}

// findAppropriatePlaces finds places where neither the previous nor the next story belongs to the given presenter.
func (s *Segment) findAppropriatePlaces(presenter string) []int {
	// If the list of stories is empty, there are no appropriate places.
	if len(s.Stories) == 0 {
		return []int{}
	}

	indexes := make([]int, 0)

	// If the first story belongs to a different presenter, the beginning is a nice position to put ours.
	if s.Stories[0].Presenter != presenter {
		indexes = append(indexes, 0)
	}

	// Same for the end.
	last := s.Stories[len(s.Stories)-1]
	if last.Presenter != presenter {
		indexes = append(indexes, len(s.Stories))
	}

	// Now, we try to find pairs of stories where neither story belongs to the given presenter.
	// Inserting our story inbetween those is a good way to ensure no presenter has two stories immediately after one another.
	for i := 0; i < len(s.Stories)-1; i++ {
		if s.Stories[i].Presenter != presenter && s.Stories[i+1].Presenter != presenter {
			indexes = append(indexes, i+1)
		}
	}
	return indexes
}

// insertStory inserts a story at one of the given indexes.
// The index to use is selected randomly.
func (s *Segment) insertStory(story *Story, candidates []int) {
	rnd := rand.Int() % len(candidates)
	idx := candidates[rnd]

	// Ensure the slice has enough capacity.
	s.Stories = append(s.Stories, nil)
	// Shift all elements after idx by 1 to the right, overwriting the just appended element.
	copy(s.Stories[idx+1:], s.Stories[idx:])

	s.Stories[idx] = story
}

func (r *repo) PresenterNames() []string {
	return r.Presenters
}

func (r *repo) migrate() error {
	if r.DBVersion == 1 {
		// Already at the latest version, no need to migrate
		return nil
	}

	// In version 0, stories didn't have IDs, so we assign them here.
	for _, e := range r.Episodes {
		for _, seg := range e.Segments {
			for i, s := range seg.Stories {
				s.ID = i + 1
			}
		}
	}

	r.DBVersion = 1

	return nil
}
