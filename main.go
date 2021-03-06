package main

import (
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"io"
	"log"
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/gomarkdown/markdown"
	"github.com/gomarkdown/markdown/html"
	"github.com/microcosm-cc/bluemonday"
)

var (
	dataDir    string
	listenAddr string

	//go:embed templates/*.html
	templateFS embed.FS
	//go:embed static/*
	staticFS embed.FS
)

func init() {
	defaultDataDir := os.Getenv("XDG_DATA_HOME")
	if defaultDataDir == "" {
		defaultDataDir = path.Join("$HOME", ".local", "share")
	}
	defaultDataDir = path.Join(defaultDataDir, "knowledge-base")
	defaultDataDir = os.ExpandEnv(defaultDataDir)

	flag.StringVar(&listenAddr, "listen-addr", ":8080", "HTTP listen address")
	flag.StringVar(&dataDir, "root", defaultDataDir, "filepath to store app data")
}

func main() {
	flag.Parse()

	mustCreateDataDir(dataDir)
	app := NewApp(dataDir, listenAddr)

	// TODO: configure server params
	log.Println("Starting HTTP server on", app.listenAddr)
	panic(http.ListenAndServe(app.listenAddr, app))
}

func mustCreateDataDir(dir string) {
	if err := os.MkdirAll(dir, 0750); err != nil {
		log.Panicf("failed to create datadir at '%s': %v", dir, err)
	}
}

const DefaultFileMode os.FileMode = 0640

type App struct {
	listenAddr string

	router    *Router
	templates *template.Template
	posts     PostsService
}

func NewApp(postsRoot, listenAddr string) *App {
	app := &App{
		listenAddr: listenAddr,
		router:     &Router{},
		posts:      NewPostsService(postsRoot),
	}

	// Templates
	app.templates = template.Must(
		template.New("all").ParseFS(templateFS, "templates/*.html"))

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

	app.router.Get(`^/api/search$`, app.SearchHandler())

	return app
}

// The main HTTP request router and handler.
func (app *App) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	app.router.ServeHTTP(w, r)
}

func (app *App) IndexHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.FormValue("q")

		posts, err := app.posts.ListPosts(&ListPostOptions{SearchTerm: q})
		if err != nil {
			http.Error(w, fmt.Sprintf("%v", err), 500)
			return
		}

		tags, err := app.posts.ListTags()
		if err != nil {
			log.Printf("error: failed to get tags: %v", err)
			tags = nil
		}

		locals := struct {
			Posts   []*Post
			AllTags []Tag
		}{
			Posts:   posts,
			AllTags: tags,
		}
		if err := app.templates.ExecuteTemplate(w, "index.html", locals); err != nil {
			log.Printf("error: template: %v", err)
		}
	}
}

type UpdatePostRequest struct {
	Title   string
	Content string
	Tags    []Tag
}

func (app *App) EditPostHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		postID := r.Context().Value("id").(string)
		post, err := app.posts.GetPost(postID)
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
			post.Tags = req.Tags

			if err := app.posts.UpdatePost(post); err != nil {
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

			if err = app.posts.CreatePost(p); err != nil {
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

		post, err := app.posts.GetPost(postID)
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
	readFile := func(path string) ([]byte, error) {
		f, err := staticFS.Open(path)
		if err != nil {
			return nil, err
		}
		defer f.Close()

		b, err := io.ReadAll(f)
		if err != nil {
			return nil, err
		}

		return b, nil
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		isStatic := strings.HasPrefix(r.URL.Path, "/static/")
		isFavicon := r.URL.Path == "/favicon.ico"

		if !isStatic && !isFavicon {
			next.ServeHTTP(w, r)
			return
		}

		path := strings.TrimPrefix(r.URL.Path, "/")
		if isFavicon {
			path = "static/favicon.ico"
		}
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

func (app *App) SearchHandler() http.HandlerFunc {
	//renderer := html.NewRenderer(
	//	html.RendererOptions{Flags: html.CommonFlags | html.HrefTargetBlank},
	//)
	// Strip all HTML after markdown, to get clear text
	//bm := bluemonday.NewPolicy()

	return func(w http.ResponseWriter, r *http.Request) {
		q := r.FormValue("q")

		posts, err := app.posts.ListPosts(&ListPostOptions{SearchTerm: q})
		if err != nil {
			http.Error(w, fmt.Sprintf("%v", err), 500)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(posts); err != nil {
			log.Printf("error: SearchHandler: %v", err)
			http.Error(w, fmt.Sprintf("%v", err), 400)
			return
		}
	}
}
