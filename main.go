package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/segmentio/ksuid"
)

func main() {
	app := &App{
		Root:       "/tmp/knowledge-base",
		ListenAddr: ":8080",
	}

	if err := app.SetupTemplates(); err != nil {
		panic(err)
	}

	// TODO: configure server params
	log.Println("Starting HTTP server on", app.ListenAddr)
	panic(http.ListenAndServe(app.ListenAddr, app))
}

const DefaultFileMode os.FileMode = 0640

type App struct {
	Root       string
	ListenAddr string

	templates *template.Template
}

type Post struct {
	ID           string
	Title        string
	Content      string
	Tags         []string
	CreatedTime  time.Time
	ModifiedTime time.Time
}

// The main HTTP request router and handler.
func (app *App) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	reqPath := r.URL.Path
	postPat := regexp.MustCompile("^/posts/([a-zA-Z0-9]{27})/?$")

	handle := func(status int, format string, args ...interface{}) {
		if format == "" {
			format = http.StatusText(status)
		}
		w.WriteHeader(status)
		fmt.Fprintf(w, format, args...)
	}
	handleLog := func(err error, status int, format string, args ...interface{}) {
		log.Printf("error: %s %s %d - %v", r.Method, reqPath, r.ContentLength, err)
		handle(status, format, args...)
	}

	if r.Method == http.MethodGet {
		// Serve static content
		if strings.HasPrefix(reqPath, "/static/") {
			staticHandler := http.StripPrefix("/static/", http.FileServer(http.Dir("./static/")))
			staticHandler.ServeHTTP(w, r)
			return
		}

		// Get home
		if reqPath == "/" {
			app.templates.ExecuteTemplate(w, "index.html", nil)
			return
		}

		// Get all posts
		if reqPath == "/posts/" {
			posts, err := app.ListPosts()
			if err != nil {
				handleLog(err, http.StatusInternalServerError, "")
			}

			b, err := json.Marshal(posts)
			if err != nil {
				handleLog(err, http.StatusInternalServerError, "")
			}

			w.Write(b)
			return
		}

		// Get post by id
		if m := postPat.FindStringSubmatch(reqPath); m != nil {
			post, err := app.GetPost(m[1])
			if err != nil {
				handle(http.StatusNotFound, "")
				return
			}

			b, err := json.Marshal(post)
			if err != nil {
				handleLog(err, http.StatusInternalServerError, "")
			}

			w.Write(b)
			return
		}

		// Get all tags
		if reqPath == "/tags/" {
			tags, err := app.ListTags()
			if err != nil {
				handleLog(err, http.StatusInternalServerError, "")
			}

			b, err := json.Marshal(tags)
			if err != nil {
				handleLog(err, http.StatusInternalServerError, "")
			}

			w.Write(b)
			return
		}
	}

	if r.Method == http.MethodPost {
		// Create post
		if reqPath == "/posts/" {
			body, err := io.ReadAll(r.Body)
			if err != nil {
				handle(http.StatusNotFound, "")
				return
			}

			p := new(Post)
			err = json.Unmarshal(body, p)
			if err != nil {
				handle(http.StatusBadRequest, "")
				return
			}

			p, err = app.CreatePost(*p)
			if err != nil {
				handle(http.StatusBadRequest, "")
				return
			}

			b, err := json.Marshal(p)
			if err != nil {
				handleLog(err, http.StatusInternalServerError, "")
			}

			w.WriteHeader(http.StatusCreated)
			w.Write(b)
			return
		}
	}

	w.WriteHeader(http.StatusNotFound)
	w.Write([]byte("Not found\n"))
}

// Initialise app HTML templates.
func (app *App) SetupTemplates() error {
	app.templates = template.Must(template.ParseGlob(filepath.Join("templates", "*.html")))
	return nil
}

// Returns a single post by ID.
func (app *App) GetPost(id string) (*Post, error) {
	filepath := path.Join(app.Root, id)
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

// Returns a list of all posts.
func (app *App) ListPosts() ([]*Post, error) {
	fileSystem := os.DirFS(app.Root)

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

		posts = append(posts, p)

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

	filepath := path.Join(app.Root, id.String())
	if err := os.WriteFile(filepath, b, DefaultFileMode); err != nil {
		return nil, fmt.Errorf("CreatePost: %w", err)
	}

	return &p, nil
}

// Updates a posts title and content. All other fields are ignored.
func (app *App) UpdatePost(patch Post) (*Post, error) {
	p, err := app.GetPost(patch.ID)
	if err != nil {
		return nil, fmt.Errorf("UpdatePost: %w", err)
	}

	p.Title = patch.Title
	p.Content = patch.Content
	p.ModifiedTime = time.Now()

	b, err := json.Marshal(p)
	if err != nil {
		return nil, fmt.Errorf("UpdatePost: %w", err)
	}

	filepath := path.Join(app.Root, p.ID)
	if err := os.WriteFile(filepath, b, DefaultFileMode); err != nil {
		return nil, fmt.Errorf("UpdatePost: %w", err)
	}

	return p, nil
}

// Returns a list of all distinct tags of all posts.
func (app *App) ListTags() ([]string, error) {
	posts, err := app.ListPosts()
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
