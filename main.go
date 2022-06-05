package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"log"
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gomarkdown/markdown"
	"github.com/gomarkdown/markdown/html"
	"github.com/microcosm-cc/bluemonday"
	"github.com/segmentio/ksuid"
)

func main() {
	app := NewApp("/tmp/knowledge-base", "static/", ":8080")

	// TODO: configure server params
	log.Println("Starting HTTP server on", app.listenAddr)
	panic(http.ListenAndServe(app.listenAddr, app))
}

const DefaultFileMode os.FileMode = 0640

type App struct {
	staticRoot string
	postsRoot  string
	listenAddr string

	router    *Router
	templates *template.Template
}

func NewApp(postsRoot, staticRoot, listenAddr string) *App {
	app := &App{
		listenAddr: listenAddr,
		postsRoot:  postsRoot,
		staticRoot: staticRoot,
		router:     &Router{},
	}

	// Templates
	app.templates = template.Must(
		template.New("all").ParseGlob(filepath.Join("templates", "*.html")))

	// Routes
	app.router.Use(app.LogHandler)
	app.router.Use(app.StaticHandler)
	app.router.Get("^/$", app.IndexHandler())

	app.router.Get("^/posts/?$", app.CreatePostHandler())
	app.router.Post("^/posts/?$", app.CreatePostHandler())
	app.router.Get(`^/posts/(?P<id>\w+)$`, app.GetPostHandler())
	app.router.Get(`^/posts/(?P<id>\w+)/edit$`, app.EditPostHandler())
	app.router.Post(`^/posts/(?P<id>\w+)/edit$`, app.EditPostHandler())
	app.router.Post(`^/render-markdown$`, app.RenderMarkdownHandler())

	return app
}

func (app *App) IndexHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.FormValue("q")

		posts, err := app.ListPosts(&ListPostOptions{SearchTerm: q})
		if err != nil {
			http.Error(w, fmt.Sprintf("%v", err), 500)
			return
		}

		locals := struct {
			Posts []*Post
		}{
			Posts: posts,
		}
		if err := app.templates.ExecuteTemplate(w, "index.html", locals); err != nil {
			log.Printf("error: template: %v", err)
		}
	}
}

type UpdatePostRequest struct {
	Title   string
	Content string
}

func (app *App) EditPostHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		postID := r.Context().Value("id").(string)
		post, err := app.GetPost(postID)
		if err != nil {
			log.Printf("error: EditPostHandler: %v", err)
			http.Error(w, fmt.Sprintf("%v", err), 404)
			return
		}

		// POST
		if r.Method == http.MethodPost {
			body, err := io.ReadAll(r.Body)
			if err != nil {
				log.Printf("error: EditPostHandler: %v", err)
				http.Error(w, fmt.Sprintf("%v", err), 400)
				return
			}

			req := new(UpdatePostRequest)
			err = json.Unmarshal(body, req)
			if err != nil {
				log.Printf("error: EditPostHandler: %v", err)
				http.Error(w, fmt.Sprintf("%v", err), 400)
				return
			}

			post.Title = req.Title
			post.Content = req.Content

			post, err := app.UpdatePost(post)
			if err != nil {
				log.Printf("error: EditPostHandler: %v", err)
				http.Error(w, fmt.Sprintf("%v", err), 400)
				return
			}

			b, err := json.Marshal(post)
			if err != nil {
				log.Printf("error: EditPostHandler: %v", err)
				http.Error(w, fmt.Sprintf("%v", err), 500)
				return
			}

			w.Header().Set("Location", "/posts/"+post.ID)
			w.WriteHeader(http.StatusCreated)
			w.Write(b)
			return
		}

		// GET
		locals := struct {
			Post *Post
		}{
			Post: post,
		}
		if err := app.templates.ExecuteTemplate(w, "post_form.html", locals); err != nil {
			log.Printf("error: template: %v", err)
		}
	}
}

func (app *App) CreatePostHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// GET
		if r.Method == http.MethodGet {
			if err := app.templates.ExecuteTemplate(w, "post_form.html", nil); err != nil {
				log.Printf("error: template: %v", err)
			}
			return
		}

		// POST
		if r.Method == http.MethodPost {
			body, err := io.ReadAll(r.Body)
			if err != nil {
				log.Printf("error: CreatePostHandler: %v", err)
				http.Error(w, fmt.Sprintf("%v", err), 400)
				return
			}

			p := new(Post)
			err = json.Unmarshal(body, p)
			if err != nil {
				log.Printf("error: CreatePostHandler: %v", err)
				http.Error(w, fmt.Sprintf("%v", err), 400)
				return
			}

			p, err = app.CreatePost(*p)
			if err != nil {
				log.Printf("error: CreatePostHandler: %v", err)
				http.Error(w, fmt.Sprintf("%v", err), 400)
				return
			}

			b, err := json.Marshal(p)
			if err != nil {
				log.Printf("error: CreatePostHandler: %v", err)
				http.Error(w, fmt.Sprintf("%v", err), 500)
				return
			}

			w.Header().Set("Location", "/posts/"+p.ID)
			w.WriteHeader(http.StatusCreated)
			w.Write(b)
			return
		}
	}
}

type GetPostResponse struct {
	*Post
	ContentHTML template.HTML
}

func (app *App) GetPostHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		postID := r.Context().Value("id").(string)

		post, err := app.GetPost(postID)
		if err != nil {
			log.Printf("error: GetPostHandler: %v", err)
			http.Error(w, fmt.Sprintf("%v", err), 400)
			return
		}

		// Render HTML from markdown
		renderer := html.NewRenderer(
			html.RendererOptions{Flags: html.CommonFlags | html.HrefTargetBlank},
		)
		s := string(markdown.ToHTML([]byte(post.Content), nil, renderer))

		// Sanitize
		bm := bluemonday.UGCPolicy()
		s = bm.Sanitize(s)

		locals := GetPostResponse{
			Post:        post,
			ContentHTML: template.HTML(s),
		}
		if err := app.templates.ExecuteTemplate(w, "post.html", locals); err != nil {
			log.Printf("error: template: %v", err)
		}
	}
}

func (app *App) LogHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s (body: %d bytes)\n", r.Method, r.URL.Path, r.ContentLength)
		next.ServeHTTP(w, r)
	})
}

func (app *App) StaticHandler(next http.Handler) http.Handler {
	cache := make(map[string][]byte)
	fs := os.DirFS(app.staticRoot)
	readFile := func(path string) ([]byte, error) {
		if b, ok := cache[path]; ok {
			return b, nil
		}

		f, err := fs.Open(path)
		if err != nil {
			// Remember that this file can't be opened for any reason
			cache[path] = nil
			return nil, err
		}
		defer f.Close()

		b, err := io.ReadAll(f)
		if err != nil {
			// Remember that this file can't be read for any reason
			cache[path] = nil
			return nil, err
		}

		cache[path] = b
		return b, nil
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/static/") {
			next.ServeHTTP(w, r)
			return
		}

		path := strings.TrimPrefix(r.URL.Path, "/static/")
		b, err := readFile(path)
		if err != nil {
			next.ServeHTTP(w, r)
			return
		}

		parts := strings.Split(filepath.Base(path), ".")
		contentType := mime.TypeByExtension("." + parts[len(parts)-1])
		if contentType == "" {
			w.Header().Set("Content-Type", "text/plain")
		} else {
			w.Header().Set("Content-Type", contentType)
		}

		_, err = w.Write(b)
		if err != nil {
			log.Printf("error: StaticHandler: %v", err)
		}
	})
}

type RenderMarkdownRequest struct {
	Markdown string
}

func (app *App) RenderMarkdownHandler() http.HandlerFunc {
	renderer := html.NewRenderer(
		html.RendererOptions{Flags: html.CommonFlags | html.HrefTargetBlank},
	)
	bm := bluemonday.UGCPolicy()

	return func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		body, err := io.ReadAll(r.Body)
		if err != nil {
			log.Printf("error: RenderMarkdownHandler: %v", err)
			http.Error(w, fmt.Sprintf("%v", err), 400)
			return
		}

		req := new(RenderMarkdownRequest)
		if err := json.Unmarshal(body, req); err != nil {
			log.Printf("error: RenderMarkdownHandler: %v", err)
			http.Error(w, fmt.Sprintf("%v", err), 400)
			return
		}

		s := string(markdown.ToHTML([]byte(req.Markdown), nil, renderer))
		s = bm.Sanitize(s)
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(w, "%s", s)
	}
}

// The main HTTP request router and handler.
func (app *App) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	app.router.ServeHTTP(w, r)
}

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

		if opts.SearchTerm != "" {
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
