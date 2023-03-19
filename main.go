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

	app.router.Get("^/posts/?$", app.PostHandler())
	app.router.Post("^/posts/?$", app.PostHandler())
	app.router.Get(`^/posts/(?P<id>\w+)$`, app.PostHandler())
	app.router.Post(`^/posts/(?P<id>\w+)$`, app.PostHandler())

	app.router.Post(`^/render-markdown$`, app.RenderMarkdownHandler())

	app.router.Get(`^/api/search$`, app.SearchHandler())

	return app
}

// The main HTTP request router and handler.
func (app *App) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	app.router.ServeHTTP(w, r)
}

type Globals struct {
	PostsTree *Node
	AllTags   []Tag
}

type Locals struct {
	Globals Globals
	Locals  any
}

func (app *App) buildLocals(extra any) *Locals {
	postsTree, err := app.posts.GetPostsFolderTree()
	if err != nil {
		log.Printf("error: failed to get posts folder tree: %v", err)
	}

	tags, err := app.posts.ListTags(&ListTagOptions{IgnoreFunctional: true})
	if err != nil {
		log.Printf("error: failed to get tags: %v", err)
		tags = nil
	}

	return &Locals{
		Globals: Globals{
			PostsTree: postsTree,
			AllTags:   tags,
		},
		Locals: extra,
	}
}

func (app *App) IndexHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		searchQ := r.FormValue("q")
		searchTags := r.FormValue("tags")

		// Post content filter
		posts, err := app.posts.ListPosts(&ListPostOptions{
			SearchTerm: searchQ,
			TagsFilter: strings.Split(searchTags, ","),
		})
		if err != nil {
			log.Printf("error: failed to list posts: %v", err)
			return
		}

		locals := app.buildLocals(struct {
			Posts        []*Post
			ContentQuery string
			TagQuery     string
		}{
			Posts:        posts,
			ContentQuery: searchQ,
			TagQuery:     searchTags,
		})

		if err := app.templates.ExecuteTemplate(w, "index.html", locals); err != nil {
			log.Printf("error: template: %v", err)
		}
	}
}

func (app *App) PostHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var (
			post *Post
			err  error
		)

		if postID, ok := r.Context().Value("id").(string); ok {
			post, err = app.posts.GetPost(postID)
			if err != nil {
				log.Printf("error: UpdatePostHandler: %v", err)
				http.Error(w, fmt.Sprintf("%v", err), 404)
				return
			}
		} else {
			post = new(Post)
		}

		if r.Method == http.MethodGet {
			locals := app.buildLocals(struct {
				Post      *Post
				IsEditing bool
			}{
				Post:      post,
				IsEditing: post.ID == "" || r.URL.Query().Has("isEditing"),
			})
			if err := app.templates.ExecuteTemplate(w, "post.html", locals); err != nil {
				log.Printf("error: template: %v", err)
			}
		} else if r.Method == http.MethodPost {

			post.Title = r.FormValue("title")
			post.Content = r.FormValue("content")
			for _, s := range strings.Split(r.FormValue("tags"), ",") {
				if len(s) > 0 {
					post.Tags = append(post.Tags, Tag(s))
				}
			}

			if post.ID == "" {
				if err := app.posts.CreatePost(post); err != nil {
					log.Printf("error: UpdatePostHandler: %v", err)
					http.Error(w, fmt.Sprintf("%v", err), 400)
					return
				}
			} else {
				if err := app.posts.UpdatePost(post); err != nil {
					log.Printf("error: UpdatePostHandler: %v", err)
					http.Error(w, fmt.Sprintf("%v", err), 400)
					return
				}
			}

			http.Redirect(w, r, "/posts/"+post.ID, http.StatusSeeOther)
			return
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

func (app *App) RenderMarkdownHandler() http.HandlerFunc {
	renderer := html.NewRenderer(
		html.RendererOptions{Flags: html.CommonFlags | html.HrefTargetBlank},
	)
	bm := bluemonday.UGCPolicy()

	return func(w http.ResponseWriter, r *http.Request) {
		md := r.FormValue("content")
		s := string(markdown.ToHTML([]byte(md), nil, renderer))
		s = bm.Sanitize(s)
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(w, "%s", s)
	}
}

func (app *App) SearchHandler() http.HandlerFunc {
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
