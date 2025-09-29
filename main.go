package main

import (
	"fmt"
	"time"
	"path/filepath"
	"strings"
	"os"
	"io"
	"io/fs"
	"log"
	"net/http"
	"html/template"

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
		article.DisplayName = ExtractRawText(heading)
		article.Title = template.HTML(markdown.Render(heading, renderer))
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

func RenderArticle(w io.Writer, article Article) {
	type templateData struct {
		DisplayName string
		Title template.HTML
		LastUpdated string
		Content template.HTML
	}

	data := templateData{
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

func RenderIndexPage(w io.Writer, title string, articles []Article){
	type templateData struct {
		ArticleList []Article
		PageTitle string
	}

	data := templateData{
		ArticleList: articles,
		PageTitle: title,
	}

	err := indexTempl.Execute(w, data)
	if err != nil {
		log.Fatal("Failed to execute template: ", err.Error())
	}
}

func main(){
	log.Println("Initializing templates")
	initTemplates()

	log.Println("Loading articles")
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
		log.Println("Loaded ", file)
	}

	log.Println("Router setup")
	router := chi.NewRouter()
	router.Use(middleware.Compress(5))
	fileServer := http.FileServer(http.Dir("./static"))

	router.Get("/", func(w http.ResponseWriter, r *http.Request){
		RenderIndexPage(w, "The Blog", articleList)
	})

	router.Handle("/static/*", http.StripPrefix("/static/", fileServer))

	router.Get("/article/{name}", func(w http.ResponseWriter, r *http.Request){
		name := chi.URLParam(r, "name")

		if article, ok := articleCache[name]; ok {
			RenderArticle(w, article)
		} else {
			http.Error(w, http.StatusText(404), 404)
		}
	})

	listenAddress := ":8080"
	log.Println("Listening on ", listenAddress)
	http.ListenAndServe(listenAddress, router)
}