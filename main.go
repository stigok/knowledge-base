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
	"regexp"
	"strings"
	"time"

	"github.com/segmentio/ksuid"
)

const DefaultFileMode os.FileMode = 0640
const PostTemplateString = `<h1>{{.Title}}</h1>
{{.Content}}`

type App struct {
	Root       string
	ListenAddr string

	postTemplate *template.Template
}

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
		// Get home
		if reqPath == "/" {
			handle(http.StatusOK, "Welcome!")
			return
		}

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

func (app *App) SetupTemplates() error {
	pt, err := template.New("post").Parse(PostTemplateString)
	if err != nil {
		return err
	}
	app.postTemplate = pt
	return nil
}

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

type Post struct {
	ID           string
	Title        string
	Content      string
	Tags         []string
	CreatedTime  time.Time
	ModifiedTime time.Time
}

func main() {
	app := &App{
		Root:       "/tmp/knowledge-base",
		ListenAddr: ":8080",
	}

	if err := app.SetupTemplates(); err != nil {
		panic(err)
	}

	// TODO: configure server params
	panic(http.ListenAndServe(app.ListenAddr, app))
}
