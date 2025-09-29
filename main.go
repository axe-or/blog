package main

import (
	"fmt"
	"time"
	"path/filepath"
	"strings"
	"os"
	"io/fs"
	"log"
	"html/template"

	"github.com/gomarkdown/markdown"
	"github.com/gomarkdown/markdown/ast"
	"github.com/gomarkdown/markdown/parser"
	"github.com/gomarkdown/markdown/html"
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

func RenderArticle(article Article) string {
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

	sb := strings.Builder{}
	err := articleTempl.Execute(&sb, data)
	if err != nil {
		log.Print("Failed to execute template: ", err.Error())
		return ""
	}

	return sb.String()
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

func main(){
	initTemplates()
	// article := NewArticle("foo", "# Hello *world*!\nparagraph", time.Now())

	articleCache := make(map[string]Article)

	mdFiles, _ := ListDirectoryMarkdownFiles("articles")
	for _, file := range mdFiles {
		article, loadError := LoadArticleFromFile(file)
		if loadError != nil {
			log.Println("Failed to load article", file, ":", loadError.Error())
		}
		articleCache[article.Name] = article
	}


	for _, article := range articleCache {
		fmt.Println("-------", article.Name,"------")
		fmt.Println(RenderArticle(article))
	}

	// fmt.Println("Name:", article.Name)
	// fmt.Println("Title:", article.Title)
	// fmt.Println("Display Name:", article.DisplayName)
	// fmt.Println("Content:", article.Content)

	// fmt.Println(RenderArticle(article))
}