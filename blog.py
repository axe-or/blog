import mistletoe as md
import sqlite3 as sql
import os.path as path
import urllib.parse
from datetime import datetime
from dataclasses import dataclass
from sys import argv, exit

DATETIME_FORMAT = "%Y-%m-%d %H:%M:%S"

def parse_datetime(s: str) -> datetime:
	return datetime.strptime(s, DATETIME_FORMAT)

@dataclass
class Article:
	name : str
	title : str
	author : str
	contents : str

	id : int = 0
	created_at : datetime = None
	updated_at : datetime = None
	deleted_at : datetime | None = None

SCHEMA_QUERY = '''
	CREATE TABLE IF NOT EXISTS Article(
		 id INTEGER PRIMARY KEY
		,name TEXT UNIQUE NOT NULL
		,title TEXT NOT NULL
		,author TEXT NOT NULL
		,contents TEXT NOT NULL

		,created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		,updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		,deleted_at DATETIME DEFAULT NULL
	)
'''

class Repository:
	def __init__(self, db_path: str):
		self.connection = sql.connect(db_path)
		self.connection.execute('PRAGMA foreign_keys = ON')
		self.connection.execute('PRAGMA journal_mode = WAL')

	def init_schema(self):
		cur = self.connection.cursor()
		cur.execute(SCHEMA_QUERY)

	def get_article(self, key: int| str, include_deleted = False) -> Article | None:
		with self.connection:
			query = '''
				SELECT
					id, name, title, author, contents, created_at, updated_at, deleted_at
				FROM
					Article
				WHERE
			'''

			if type(key) is str:
				query += 'name = ?'
			elif type(key) is int:
				query += 'id = ?'
			else:
				raise TypeError("Invalid key type")

			cur = self.connection.execute(query, (key,))

			res = cur.fetchone()
			if res is None:
				return None

			(id, name, title, author, contents, created_at_str, updated_at_str, deleted_at_str) = res
			updated_at = parse_datetime(updated_at_str)
			created_at = parse_datetime(created_at_str)
			deleted_at = None
			if deleted_at_str:
				deleted_at = parse_datetime(deleted_at_str)
				if not include_deleted:
					return None

			return Article(id=id, name=name, title=title, author=author, contents=contents,
				created_at=created_at, updated_at=updated_at, deleted_at=deleted_at)

	def list_articles(self, include_deleted = False) -> list[Article]:
		articles = []

		with self.connection:
			query = '''
				SELECT
					id, name, title, author, contents, created_at, updated_at, deleted_at
				FROM
					Article
			'''

			if not include_deleted:
				query += ' WHERE deleted_at IS NULL'

			cur = self.connection.execute(query)

			for res in cur:
				(id, name, title, author, contents, created_at_str, updated_at_str, deleted_at_str) = res
				updated_at = parse_datetime(updated_at_str)
				created_at = parse_datetime(created_at_str)
				deleted_at = None

				if deleted_at_str:
					deleted_at = parse_datetime(deleted_at_str)

				article = Article(id=id, name=name, title=title, author=author, contents=contents,
					created_at=created_at, updated_at=updated_at, deleted_at=deleted_at)

				articles.append(article)

		return articles

	def create_article(self, article: Article):
		cur = None

		with self.connection:
			self.connection.execute('''
				INSERT INTO
					Article(name, title, author, contents)
				VALUES
					(?, ?, ?, ?)
			''', (article.name, article.title, article.author, article.contents))

	def update_article(self, article: Article):
		with self.connection:
			self.connection.execute('''
				UPDATE
					Article
				SET
				 name = ?
				,title = ?
				,author = ?
				,contents = ?
				,updated_at = current_timestamp
			WHERE
				id = ?
			''', (article.name, article.title, article.author, article.contents, article.id))

	def restore_article(self, key: int | str):
		get_article_by_name()

	def delete_article(self, key: int | str):
		with self.connection:
			query = '''
				UPDATE
					Article
				SET
					deleted_at = CURRENT_TIMESTAMP
				WHERE
			'''

			if type(key) is str:
				query += 'name = ?'
			elif type(key) is int:
				query += 'id = ?'
			else:
				raise TypeError("Invalid key type")

			self.connection.execute(query, (key,))

	def purge_deleted_entries(self):
		with self.connection:
			self.connection.execute('''
				DELETE FROM
					Article
				WHERE
					deleted_at IS NOT NULL
			''')

def is_url_path_safe(name: str) -> bool:
    return name == urllib.parse.quote(name, safe='_.-~')


HELP_MSG = f'''
Usage: {argv[0]} <command>

Commands:
    publish <file>   Publish a .md <file>
    delete <name>    Remove article with <name>
    generate <dir>   Generate a static site rooted at <dir>
    list             List all created articles
    purge            Purge all articles marked as deleted
'''.strip()

def main():
	repo = Repository('app.db')
	repo.init_schema()

	try:
		cmd = argv[1]

		if cmd == 'publish':
			filepath = argv[2]
			name = path.splitext(path.basename(filepath))[0].strip()
			if len(name) == 0 or not is_url_path_safe(name):
				print(f'Error: file name {filepath} is not URL path safe')
				exit(1)

			filedata = ''
			with open(filepath, 'r') as f:
				filedata = f.read()

			article = Article(name=name, author='meeee', contents=filedata, title = 'something')
			repo.create_article(article)

		elif cmd == 'generate':
			article = repo.get_article('skibidi')
			print(md.markdown(article.contents))
		elif cmd == 'delete':
			pass
		else:
			print(HELP_MSG)
			exit(1)
	except IndexError:
		print(HELP_MSG)
		exit(1)

if __name__ == '__main__': main()
