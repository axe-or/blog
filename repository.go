package main

import (
	"log"
	"time"
	"errors"
	_ "embed"

	_ "github.com/ncruces/go-sqlite3/driver"
	_ "github.com/ncruces/go-sqlite3/embed"

	"github.com/jmoiron/sqlx"
)

type Article struct {
	Id int64
	Name string
	Title string
	Content string
	UpdatedAt time.Time
	CreatedAt time.Time
}

type Repository struct {
	db *sqlx.DB
}

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

func (repo *Repository) CreateArticle(name, title, content string) (id int64, err error) {
	res, err := repo.db.Exec(`
		INSERT INTO Article(
			Name, Title, Content, CreatedAt, UpdatedAt
		)
		VALUES (
			?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
		)
	`, name, title, content)

	if err != nil {
		return -1, err
	}

	id, err = res.LastInsertId()
	return
}

func (repo *Repository) GetArticleByName(name string) (Article, error){
	rows, err := repo.db.Queryx(`
		SELECT
			*
		FROM
			Article
		WHERE
			Name = ?
	`, name)

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

var IdNotFoundErr error = errors.New("id does not exist")

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

//go:embed schema.sql
var DB_SCHEMA string

func main(){
	repo, err := NewRepository("blog.db")
	if err != nil {
		log.Fatal(err.Error())
	}

	articles, err := repo.ListArticles()
	for _, a := range articles {
		log.Println(a.Name, a.CreatedAt)
	}
}
