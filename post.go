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

type PostsService interface {
	GetPost(id string) (*Post, error)
	ListPosts(opts *ListPostOptions) ([]*Post, error)
	UpdatePost(post *Post) error
	CreatePost(post *Post) error
	ListTags(opts *ListTagOptions) ([]Tag, error)
	GetPostsFolderTree() (*Node, error)
}

type postsService struct {
	// Path to directory where posts are stored
	root string
}

func NewPostsService(root string) PostsService {
	return &postsService{
		root: root,
	}
}

type Tag string

type Post struct {
	ID           string
	Title        string
	Content      string
	Tags         []Tag
	CreatedTime  time.Time
	ModifiedTime time.Time
}

// Returns a single post by ID.
func (svc postsService) GetPost(id string) (*Post, error) {
	filepath := path.Join(svc.root, id)
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
	TagsFilter []string
}

// Returns a list of all posts.
func (svc postsService) ListPosts(opts *ListPostOptions) ([]*Post, error) {
	fileSystem := os.DirFS(svc.root)

	filters := []func(*Post) bool{
		// Filter on search term
		func(p *Post) bool {
			if opts.SearchTerm == "" {
				return true
			}

			return strings.Contains(p.Title, opts.SearchTerm) ||
				strings.Contains(p.Content, opts.SearchTerm)
		},
		// Filter on tags
		func(p *Post) bool {
			if len(opts.TagsFilter) == 0 {
				return true
			}
			for _, t := range opts.TagsFilter {
				for _, pt := range p.Tags {
					if string(pt) == t {
						return true
					}
				}
			}
			return false
		},
	}

	var posts []*Post

	if opts == nil {
		opts = &ListPostOptions{}
	}

	err := fs.WalkDir(fileSystem, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if path == "." {
			return nil
		}

		id := strings.TrimSuffix(strings.TrimPrefix(path, "./"), ".json")
		p, err := svc.GetPost(id)
		if err != nil {
			return err
		}

		for _, pred := range filters {
			if !pred(p) {
				break
			}
		}
		posts = append(posts, p)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("ListPosts: %w", err)
	}

	return posts, nil
}

// Create a post. ID, CreatedTime and ModifiedTime will be overwritten if present.
func (svc postsService) CreatePost(p *Post) error {
	now := time.Now()
	id, err := ksuid.NewRandomWithTime(now)
	if err != nil {
		return fmt.Errorf("CreatePost: %w", err)
	}

	p.ID = id.String()
	p.CreatedTime = now
	p.ModifiedTime = now

	b, err := json.Marshal(p)
	if err != nil {
		return fmt.Errorf("CreatePost: %w", err)
	}

	filepath := path.Join(svc.root, id.String())
	if err := os.WriteFile(filepath, b, DefaultFileMode); err != nil {
		return fmt.Errorf("CreatePost: %w", err)
	}

	return nil
}

// Updates a posts title and content. All other fields are ignored.
func (svc postsService) UpdatePost(p *Post) error {
	// Make sure it exists
	if _, err := svc.GetPost(p.ID); err != nil {
		return fmt.Errorf("UpdatePost: %w", err)
	}

	p.ModifiedTime = time.Now()

	b, err := json.Marshal(p)
	if err != nil {
		return fmt.Errorf("UpdatePost: %w", err)
	}

	filepath := path.Join(svc.root, p.ID)
	if err := os.WriteFile(filepath, b, DefaultFileMode); err != nil {
		return fmt.Errorf("UpdatePost: %w", err)
	}

	return nil
}

type ListTagOptions struct {
	// Ignore tags with a functional meaning.
	IgnoreFunctional bool
}

// Returns a list of all distinct tags of all posts.
func (svc postsService) ListTags(opts *ListTagOptions) ([]Tag, error) {
	if opts == nil {
		opts = new(ListTagOptions)
	}

	posts, err := svc.ListPosts(nil)
	if err != nil {
		return nil, fmt.Errorf("ListTags: %w", err)
	}

	uniqueTags := make(map[Tag]bool)
	for _, p := range posts {
		for _, tag := range p.Tags {
			if opts.IgnoreFunctional && strings.HasPrefix(string(tag), "_") {
				continue
			}
			uniqueTags[tag] = true
		}
	}

	var tags []Tag
	for tag := range uniqueTags {
		tags = append(tags, tag)
	}
	sort.Slice(tags, func(i, j int) bool {
		return tags[i] < tags[j]
	})

	return tags, nil
}

func (svc postsService) GetPostsFolderTree() (*Node, error) {
	posts, err := svc.ListPosts(nil)
	if err != nil {
		return nil, fmt.Errorf("GetPostsFolderTree: %w", err)
	}

	folders := make(map[string][]*Post)
	for _, p := range posts {
		for _, t := range p.Tags {
			parts := strings.SplitN(string(t), ":", 2)
			if len(parts) == 2 && parts[0] == "_dir" {
				folders[parts[1]] = append(folders[parts[1]], p)
			}
		}
	}

	return BuildTree(folders), nil
}
