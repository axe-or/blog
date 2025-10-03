package main

import (
	"fmt"
	"time"
	"path/filepath"
	"strings"
	"os"
	"os/signal"
	"io"
	"io/fs"
	"log"
	"net/http"
	"html/template"
	"sync"

	"github.com/gomarkdown/markdown"
	"github.com/gomarkdown/markdown/ast"
	"github.com/gomarkdown/markdown/parser"
	"github.com/gomarkdown/markdown/html"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

const ARTICLE_ROOT = "articles"

type Article struct {
    Name string
    DisplayName string
    Title template.HTML
    Content template.HTML
    UpdatedAt time.Time
}

const markdownExtensions = parser.NoIntraEmphasis | parser.Tables | parser.FencedCode |
	parser.Autolink | parser.Strikethrough | parser.SpaceHeadings | parser.HeadingIDs |
	parser.BackslashLineBreak | parser.DefinitionLists | parser.AutoHeadingIDs

func remove[T any](s []T, i int) []T {
	return append(s[:i], s[i+1:]...)
}

func PopFirstHeading(doc *ast.Document) (heading *ast.Heading) {
	for i, child := range doc.Children {
		if h, ok := child.(*ast.Heading); ok {
			heading = h
			doc.Children = remove(doc.Children, i)
			break
		}
	}

	return
}

func NewArticle(name string, source string, updatedAt time.Time) Article {
	article := Article{
		Name: name,
		Title: template.HTML(fmt.Sprintf("<h1>%s</h1>", string(name))),
		DisplayName: name,
		UpdatedAt: updatedAt,
	}

	parser := parser.NewWithExtensions(markdownExtensions)

	root := markdown.Parse([]byte(source), parser).(*ast.Document)

	opts := html.RendererOptions{Flags: html.CommonFlags | html.HrefTargetBlank}
	renderer := html.NewRenderer(opts)

	heading := PopFirstHeading(root)
	if heading != nil {
		hRoot := ast.Document{}
		hRoot.Children = make([]ast.Node, len(heading.Children))
		copy(hRoot.Children, heading.Children)

		article.DisplayName = ExtractRawText(heading)
		article.Title = template.HTML(markdown.Render(&hRoot, renderer))
	}

	article.Content = template.HTML(markdown.Render(root, renderer))

	return article
}

func extractRawTextRec(node ast.Node, sb *strings.Builder){
	if node == nil {
		return
	}

	if leaf := node.AsLeaf(); leaf != nil {
		sb.Write(leaf.Literal)
		sb.WriteRune(' ')
	}

	if container := node.AsContainer(); container != nil {
		for _, child := range container.Children {
			extractRawTextRec(child, sb)
		}
	}
}

func ExtractRawText(node ast.Node) string {
	sb := strings.Builder{}
	extractRawTextRec(node, &sb)
	return strings.TrimSpace(sb.String())
}

func RenderHTML(source string) string {
	extensions := parser.CommonExtensions | parser.AutoHeadingIDs
	p := parser.NewWithExtensions(extensions)

	opts := html.RendererOptions{Flags: html.CommonFlags | html.HrefTargetBlank}
	renderer := html.NewRenderer(opts)

	data := []byte(source)
	html := markdown.ToHTML(data, p, renderer)

	return string(html);
}

func ListDirectoryMarkdownFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil { return nil, err }

	result := make([]string, 0, len(entries))

	for _, entry := range entries {
		name := entry.Name()
		if strings.HasSuffix(name, ".md") && entry.Type().IsRegular(){
			result = append(result, filepath.Join(dir, name))
		}
	}
	return result, nil
}


var articleTempl *template.Template
var indexTempl *template.Template

func initTemplates(){
	var err error

	articleTempl, err = template.ParseFiles("templates/article.html")
	if err != nil {
		log.Fatal("Failed to initialize templates: ", err.Error())
	}

	indexTempl, err = template.ParseFiles("templates/index.html")
	if err != nil {
		log.Fatal("Failed to initialize templates: ", err.Error())
	}
}

type articleTemplateData struct {
	DisplayName string
	Name string
	Title template.HTML
	LastUpdated string
	Content template.HTML
}


func RenderArticle(w io.Writer, article Article) {
	data := articleTemplateData{
		DisplayName: article.DisplayName,
		Title: article.Title,
		Content: article.Content,
		LastUpdated: article.UpdatedAt.Format("2006-01-02"),
	}

	err := articleTempl.Execute(w, data)
	if err != nil {
		log.Fatal("Failed to execute template: ", err.Error())
	}
}

func LoadArticleFromFile(path string) (article Article, err error) {
	var (
		data []byte
		info fs.FileInfo
	)

	data, err = os.ReadFile(path)
	if err != nil { return }

	info, err = os.Stat(path)
	if err != nil { return }

	basename := filepath.Base(path)
	ext := filepath.Ext(basename)
	name := basename[:len(basename) - len(ext)]

	article = NewArticle(name, string(data), info.ModTime())
	return
}

func Apply[U, T any](f func(T) U, s []T) []U {
	res := make([]U, len(s))
	for i, v := range s {
		res[i] = f(v)
	}
	return res
}

func RenderIndexPage(w io.Writer, title string, articles []Article){
	type templateData struct {
		ArticleList []articleTemplateData
		PageTitle string
	}

	articleData := Apply(func(a Article) articleTemplateData {
		return articleTemplateData {
			DisplayName: a.DisplayName,
			Title: a.Title,
			Name: a.Name,
			Content: a.Content,
			LastUpdated: a.UpdatedAt.Format("2006-01-02"),
		}
	}, articles)

	data := templateData{
		ArticleList: articleData,
		PageTitle: title,
	}

	err := indexTempl.Execute(w, data)
	if err != nil {
		log.Fatal("Failed to execute template: ", err.Error())
	}
}


type Repository struct {
	articles    map[string]Article
	articleList []Article
	lastRefresh time.Time
	mutex       sync.RWMutex
}

func NewRepository() *Repository {
	repo := Repository {
		articles: make(map[string]Article),
		articleList: make([]Article, 0, 8),
	}
	return &repo
}

func (repo *Repository) GetArticle(name string) (Article, bool) {
	repo.mutex.RLock()
	defer repo.mutex.RUnlock()

	a, ok := repo.articles[name]
	return a, ok
}

func (repo *Repository) GetArticleList() []Article {
	repo.mutex.RLock()
	defer repo.mutex.RUnlock()

	return repo.articleList
}

func (repo *Repository) Refresh() {
	mdFiles, _ := ListDirectoryMarkdownFiles("articles")
	articleCache := make(map[string]Article, len(mdFiles))
	articleList := make([]Article, 0, len(mdFiles))

	for _, file := range mdFiles {
		article, loadError := LoadArticleFromFile(file)
		if loadError != nil {
			log.Println("Failed to load article", file, ":", loadError.Error())
			continue
		}
		articleCache[article.Name] = article
		articleList = append(articleList, article)
	}

	repo.mutex.Lock()
	defer repo.mutex.Unlock()

	repo.articles = articleCache
	repo.articleList = articleList
	repo.lastRefresh = time.Now()
}

func main(){
	log.Println("Initializing templates")
	initTemplates()

	log.Println("Loading articles")

	log.Println("Router setup")
	router := chi.NewRouter()
	router.Use(middleware.Compress(5))
	fileServer := http.FileServer(http.Dir("./static"))

	repo := NewRepository()
	go func(){
		for {
			initTemplates()
			repo.Refresh()
			time.Sleep(time.Millisecond * 1_000)
		}
	}()


	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func(){
	    for _ = range c {
	    	log.Println("Shutting down...")
	    	os.Exit(0)
	    }
	}()

	router.Get("/", func(w http.ResponseWriter, r *http.Request){
		RenderIndexPage(w, "The Blog", repo.GetArticleList())
	})

	router.Handle("/static/*", http.StripPrefix("/static/", fileServer))

	router.Get("/article/{name}", func(w http.ResponseWriter, r *http.Request){
		name := chi.URLParam(r, "name")

		if article, ok := repo.GetArticle(name); ok {
			RenderArticle(w, article)
		} else {
			http.Error(w, http.StatusText(404), 404)
		}
	})

	listenAddress := ":8080"
	log.Println("Listening on ", listenAddress)
	http.ListenAndServe(listenAddress, router)
}