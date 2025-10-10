package main

import (
	"log"
	"time"
	"errors"
	"os"
	"fmt"
	"path/filepath"
	"strings"
	"database/sql"
	"html/template"
	_ "embed"

	_ "github.com/mattn/go-sqlite3"

	"github.com/gomarkdown/markdown"
	"github.com/gomarkdown/markdown/ast"
	"github.com/gomarkdown/markdown/parser"
	"github.com/gomarkdown/markdown/html"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/jmoiron/sqlx"
)

type Article struct {
	Id int64
	Name string
	Title HTML
	RawTitle string
	Content HTML
	UpdatedAt time.Time
	CreatedAt time.Time
}

type Repository struct {
	db *sqlx.DB
}

var IdNotFoundErr error = errors.New("id does not exist")

//go:embed schema.sql
var DB_SCHEMA string

func NewRepository(dbConn string) (*Repository, error){
	repo := &Repository{}
	db, err := sqlx.Open("sqlite3", dbConn)
	if err != nil { return nil, err }

	db.MapperFunc(func (s string) string {
		return s
	})
	db.MustExec(DB_SCHEMA)

	repo.db = db
	return repo, nil
}

func (repo *Repository) Close(){
	repo.db.Close()
}

func (repo *Repository) CreateArticle(article Article) (id int64, err error) {
	res, err := repo.db.Exec(`
		INSERT INTO Article(
			Name, Title, RawTitle, Content,
			CreatedAt, UpdatedAt
		)
		VALUES (
			?, ?, ?, ?,
			CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
		)
	`, article.Name, article.Title, article.RawTitle, article.Content)

	if err != nil {
		return -1, err
	}

	id, err = res.LastInsertId()
	return
}

func (repo *Repository) GetArticleByName(name string) (Article, error){
	article := Article{}

	err := repo.db.QueryRowx(`
		SELECT
			*
		FROM
			Article
		WHERE
			Name = ?
	`, name).StructScan(&article)

	return article, err
}

func (repo *Repository) GetArticleById(id int64) (Article, error){
	rows, err := repo.db.Queryx(`
		SELECT
			*
		FROM
			Article
		WHERE
			Id = ?
	`, id)

	article := Article{}

	if err != nil {
		return article, err
	}

	for rows.Next() {
		err = rows.StructScan(&article)
		break
	}

	return article, err
}
func (repo *Repository) UpdateArticle(article Article) error {
	res, err := repo.db.Exec(`
		UPDATE
			Article
		SET
			 Name = ?
			,Title = ?
			,Content = ?
			,UpdatedAt = CURRENT_TIMESTAMP
		WHERE
			Id = ?
	`, article.Name, article.Title, article.Content, article.Id)

	if err != nil {
		return err
	}

	if count, _ := res.RowsAffected(); count < 1 {
		return IdNotFoundErr
	}

	return nil
}

func (repo *Repository) DeleteArticle(article Article) error {
	_, err := repo.db.Exec(`
		DELETE FROM
			Article
		WHERE
			Id = ?
	`, article.Id)

	return err
}

func (repo *Repository) ListArticles() ([]Article, error){
	rows, err := repo.db.Queryx(`
		SELECT
			*
		FROM
			Article
	`)

	if err != nil {
		return nil, err
	}

	articles := make([]Article, 0, 8)

	for rows.Next(){
		article := Article{}

		err = rows.StructScan(&article)
		if err != nil {
			return nil, err
		}

		articles = append(articles, article)
	}

	return articles, nil
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

func RenderMarkdownToHtml(source string) string {
	extensions := parser.CommonExtensions | parser.AutoHeadingIDs
	p := parser.NewWithExtensions(extensions)

	opts := html.RendererOptions{Flags: html.CommonFlags | html.HrefTargetBlank}
	renderer := html.NewRenderer(opts)

	data := []byte(source)
	html := markdown.ToHTML(data, p, renderer)

	return string(html);
}

func LoadArticlesFromDirectory(dirpath string, repo *Repository) error {
	files, err := ListDirectoryMarkdownFiles(dirpath)
	if err != nil { return err }

	for _, file := range files {
		article, err := LoadArticleFromFile(file)
		if err != nil {
			log.Println("Error loading file:", err.Error())
			continue
		}

		if dbArticle, err := repo.GetArticleByName(article.Name); err == nil {
			log.Println("Update", article.Name)

			article.Id = dbArticle.Id
			err := repo.UpdateArticle(article)
			if err != nil {
				log.Println("Failed to update article", err.Error())
				continue
			}

		} else if err == sql.ErrNoRows {
			log.Println("Create", article.Name)

			_, err := repo.CreateArticle(article)
			if err != nil {
				log.Println("Failed to create article", err.Error())
				continue
			}

		} else {
			log.Fatal(err.Error())
			continue
		}

	}

	return nil
}

func ArticleFromMarkdown(name string, source string) Article {
	article := Article{
		Name: name,
		RawTitle: name,
		Title: template.HTML(name),
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

		article.Title = template.HTML(markdown.Render(&hRoot, renderer))
		article.RawTitle = ExtractRawText(&hRoot)
	}

	article.Content = template.HTML(markdown.Render(root, renderer))

	return article
}

func ExtractRawText(node ast.Node) string {
	sb := strings.Builder{}
	extractRawTextRec(node, &sb)
	return strings.TrimSpace(sb.String())
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

type HTML = template.HTML

func LoadArticleFromFile(path string) (article Article, err error) {
	var data []byte

	data, err = os.ReadFile(path)
	if err != nil { return }

	basename := filepath.Base(path)
	ext := filepath.Ext(basename)
	name := basename[:len(basename) - len(ext)]

	article = ArticleFromMarkdown(name, string(data))
	return
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

//go:embed article.html
var articleTemplateData []byte

//go:embed index.html
var indexTemplateData []byte

//go:embed style.css
var styleSheetData []byte

func InitProjectTree(baseDir string) error {
	dirs := []string {
		"templates",
		"articles",
		"static",
	}

	defaultFiles := map[string][]byte {
		"templates/index.html": indexTemplateData,
		"templates/article.html": articleTemplateData,
		"static/style.css": styleSheetData,
	}

	for _, dir := range dirs {
		p := filepath.Join(baseDir, dir)
		log.Println("Create", p)
		err := os.MkdirAll(p, 0o644)
		if err != nil {
			log.Println("Failed to create directory: ", err.Error())
			return err
		}
	}

	for path, data := range defaultFiles {
		// TODO: check if exists
		p := filepath.Join(baseDir, path)
		log.Println("Create", p)
		err := os.WriteFile(p, data, 0o644)

		if err != nil {
			log.Println("Failed to create a default file: ", err.Error())
			return err
		}
	}

	return nil
}

func PrintHelp(){
	lines := []string {
		"usage: blog <command> [args]",
		"",
		"commands:",
		"  init            initialize a blog on current working directory",
		"  serve <addr>    serve blog at current directory on <addr>",
	}

	for _, line := range lines {
		fmt.Println(line)
	}
}

func getCLIArg(idx int) string {
	if idx >= len(os.Args) {
		PrintHelp()
		os.Exit(1)
	}
	return os.Args[idx]
}

func Serve(address string) *chi.Router {
	log.Println("Router setup")
	router := chi.NewRouter()
	router.Use(middleware.Compress(5))
	fileServer := http.FileServer(http.Dir("./static"))

	repo := NewRepository()


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

	router.Get("/timestamps", func(w http.ResponseWriter, r *http.Request){
		w.Header().Set("Content-Type", "application/json")
		w.Write(repo.ExportPublishingTimestamps())
	})

	return router
}

func spawnKeyboardInterruptHandler(){
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func(){
	    for _ = range c {
	    	log.Println("Shutting down")
	    	os.Exit(0)
	    }
	}()
}

func main(){
	cmd := getCLIArg(1)
	spawnKeyboardInterruptHandler()

	switch cmd {
	case "init":
		InitProjectTree(".")
	
	case "serve":
		addr := getCLIArg(2)

		log.Println("Intialize database")
		repo, err := NewRepository("blog.db")
		if err != nil {
			log.Fatal(err.Error())
		}
		log.Println("Load articles")
		LoadArticlesFromDirectory("articles", repo)

	default:
		PrintHelp()
		os.Exit(1)
	}
}
