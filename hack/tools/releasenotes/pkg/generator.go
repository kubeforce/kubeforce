package pkg

import (
	"bytes"
	_ "embed"
	"fmt"
	"os"
	"regexp"
	"strings"
	"text/template"

	"github.com/pkg/errors"
	"github.com/tsuyoshiwada/go-gitlog"
)

var (
	tagRegex       = regexp.MustCompile(`\[release-[\w-.]*]`)
	refRegex       = regexp.MustCompile(`\((#\d+)\)`)
	unsortedHeader = ":question: Sort these by hand"

	//go:embed RELEASE_NOTES.tpl.md
	releaseNotesTplFile string

	releaseNotesTemplate = template.Must(template.New("release_notes").Parse(releaseNotesTplFile))
)

type Generator struct {
	cfg Config
}

func NewGenerator(cfg Config) *Generator {
	return &Generator{
		cfg: cfg,
	}
}

func (g *Generator) Run() error {
	releaseNotes, err := g.getReleaseNotes()
	if err != nil {
		return errors.WithStack(err)
	}
	buf := bytes.NewBufferString("")
	if err := releaseNotesTemplate.Execute(buf, releaseNotes); err != nil {
		return errors.WithStack(err)
	}
	if err := os.WriteFile(g.cfg.Output, buf.Bytes(), 0o644); err != nil {
		return errors.Wrap(err, "unable to write the output file")
	}
	return nil
}

func (g *Generator) getReleaseNotes() (*ReleaseNotes, error) {
	git := gitlog.New(&gitlog.Config{
		Path: ".",
	})
	gitRange := &gitlog.RevRange{
		Old: g.cfg.GitRange.From,
		New: g.cfg.GitRange.To,
	}
	commits, err := git.Log(gitRange, nil)
	if err != nil {
		return nil, errors.Wrap(err, "unable to get commits")
	}
	fmt.Printf("git log %s. commits: %d", gitRange.Args(), len(commits))
	noteGroupMap := make(map[string]*NoteGroup)
	for _, c := range commits {
		title, note := g.parseCommit(c)
		group, ok := noteGroupMap[title]
		if !ok {
			group = &NoteGroup{
				Title: title,
				Notes: make([]Note, 0),
			}
			noteGroupMap[title] = group
		}
		group.Notes = append(group.Notes, note)
	}
	result := &ReleaseNotes{
		NoteGroups: make([]*NoteGroup, 0),
	}
	for _, h := range g.cfg.Headers {
		if group, ok := noteGroupMap[h.Name]; ok {
			result.NoteGroups = append(result.NoteGroups, group)
		}
	}
	if group, ok := noteGroupMap[unsortedHeader]; ok {
		result.NoteGroups = append(result.NoteGroups, group)
	}
	return result, nil
}

func (g *Generator) parseCommit(c *gitlog.Commit) (string, Note) {
	for _, h := range g.cfg.Headers {
		for _, p := range h.Prefixes {
			if strings.HasPrefix(c.Subject, p) {
				return h.Name, Note{
					Subject: trimTitle(strings.TrimPrefix(c.Subject, p)),
					Body:    c.Body,
					Refs:    getRefs(c.Subject),
				}
			}
		}
	}
	return unsortedHeader, Note{
		Subject: trimTitle(c.Subject),
		Body:    c.Body,
		Refs:    getRefs(c.Subject),
	}
}

func trimTitle(title string) string {
	title = tagRegex.ReplaceAllString(title, "")
	title = refRegex.ReplaceAllString(title, "")
	return strings.TrimSpace(title)
}

func getRefs(title string) []string {
	var result []string
	match := refRegex.FindAllStringSubmatch(title, -1)
	for _, gr := range match {
		result = append(result, gr[1])
	}
	return result
}
