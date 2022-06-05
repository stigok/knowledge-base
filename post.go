package main

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/segmentio/ksuid"
)

type Post struct {
	ID           string
	Title        string
	Content      string
	Tags         []string
	CreatedTime  time.Time
	ModifiedTime time.Time
}

// Returns a single post by ID.
func (app *App) GetPost(id string) (*Post, error) {
	filepath := path.Join(app.postsRoot, id)
	b, err := os.ReadFile(filepath)
	if err != nil {
		return nil, fmt.Errorf("GetPost: %w", err)
	}

	post := new(Post)
	if err := json.Unmarshal(b, post); err != nil {
		return nil, fmt.Errorf("GetPost: %w", err)
	}

	return post, nil
}

type ListPostOptions struct {
	SearchTerm string
}

// Returns a list of all posts.
func (app *App) ListPosts(opts *ListPostOptions) ([]*Post, error) {
	fileSystem := os.DirFS(app.postsRoot)

	var posts []*Post

	err := fs.WalkDir(fileSystem, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if path == "." {
			return nil
		}

		id := strings.TrimSuffix(strings.TrimPrefix(path, "./"), ".json")
		p, err := app.GetPost(id)
		if err != nil {
			return err
		}

		if opts != nil && opts.SearchTerm != "" {
			if strings.Contains(p.Title, opts.SearchTerm) || strings.Contains(p.Content, opts.SearchTerm) {
				posts = append(posts, p)

			}
		} else {
			posts = append(posts, p)
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("ListPosts: %w", err)
	}

	return posts, nil
}

// Create a post. ID, CreatedTime and ModifiedTime will be overwritten if present.
func (app *App) CreatePost(p Post) (*Post, error) {
	now := time.Now()
	id, err := ksuid.NewRandomWithTime(now)
	if err != nil {
		return nil, fmt.Errorf("CreatePost: %w", err)
	}

	p.ID = id.String()
	p.CreatedTime = now
	p.ModifiedTime = now

	b, err := json.Marshal(p)
	if err != nil {
		return nil, fmt.Errorf("CreatePost: %w", err)
	}

	filepath := path.Join(app.postsRoot, id.String())
	if err := os.WriteFile(filepath, b, DefaultFileMode); err != nil {
		return nil, fmt.Errorf("CreatePost: %w", err)
	}

	return &p, nil
}

// Updates a posts title and content. All other fields are ignored.
func (app *App) UpdatePost(p *Post) (*Post, error) {
	// Make sure it exists
	if _, err := app.GetPost(p.ID); err != nil {
		return nil, fmt.Errorf("UpdatePost: %w", err)
	}

	p.ModifiedTime = time.Now()

	b, err := json.Marshal(p)
	if err != nil {
		return nil, fmt.Errorf("UpdatePost: %w", err)
	}

	filepath := path.Join(app.postsRoot, p.ID)
	if err := os.WriteFile(filepath, b, DefaultFileMode); err != nil {
		return nil, fmt.Errorf("UpdatePost: %w", err)
	}

	return p, nil
}

// Returns a list of all distinct tags of all posts.
func (app *App) ListTags() ([]string, error) {
	posts, err := app.ListPosts(nil)
	if err != nil {
		return nil, fmt.Errorf("ListTags: %w", err)
	}

	uniqueTags := make(map[string]bool)
	for _, p := range posts {
		for _, tag := range p.Tags {
			uniqueTags[tag] = true
		}
	}

	var tags []string
	for tag, _ := range uniqueTags {
		tags = append(tags, tag)
	}
	sort.Strings(tags)

	return tags, nil
}
