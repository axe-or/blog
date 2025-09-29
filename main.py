import os
import time
import os.path as path
from sys import exit
from threading import Thread
from dataclasses import dataclass
from datetime import datetime
from mistletoe import HtmlRenderer, Document
from mistletoe.token import Token
from mistletoe.block_token import Heading
from mistletoe.span_token import RawText
from flask import Flask, render_template, make_response, abort

app = Flask(__name__)

ARTICLE_ROOT = 'articles'

@dataclass
class Article:
    name : str
    display_name : str
    title : str
    contents : str
    updated_at : datetime

    def __init__(self, name: str, source: str, updated_at: datetime | None = None):
        if updated_at is None:
            updated_at = datetime.now()

        with HtmlRenderer() as renderer:
            document = Document(source)
            title = f'<h1>{name}</h1>'
            display_name = name

            heading = pop_first_heading(document)

            if heading is not None:
                title = renderer.render(heading)
                display_name = ' '.join(extract_raw_text(heading))

            self.name = name
            self.display_name = display_name
            self.title = title
            self.contents = renderer.render(document)
            self.updated_at = updated_at

class ArticleCache:
    def __init__(self, root: str, lifespan: float):
        self.articles = {}
        self.last_reload = datetime.now()
        self.lifespan = lifespan
        self.root = root

    def reload_articles(self):
        self.last_reload = datetime.now()
        self.articles = {}
        filepaths = map(lambda p: path.join(self.root, p), filter(lambda p: p.endswith('.md'), os.listdir(self.root)))

        for filepath in filepaths:
            filedata = ''
            with open(filepath, 'r') as f:
                filedata = f.read()

            name, _ = path.splitext(path.basename(filepath))
            article = Article(name, filedata)

            self.articles[name] = article

    def get(self, name: str):
        # now = datetime.now()
        # elapsed = (now - self.last_reload).total_seconds()

        # if elapsed > self.lifespan:
        #     self.reload_articles()

        try:
            a = self.articles[name]
            return a
        except KeyError:
            return None

article_cache : ArticleCache = None

def find_first_heading(root: Token) -> Heading | None:
    if type(root) is Heading:
        if root.level == 1:
            return root

    if root.children is None:
        return None

    for child in root.children:
        res = find_first_heading(child)
        if res is not None:
            return res

def pop_first_heading(doc: Document) -> Heading | None:
    for i, child in enumerate(doc.children):
        if type(child) is Heading:
            if child.level == 1:
                doc.children.pop(i)
                return child

def _extract_raw_text_rec(node: Token, buf: list[str]):
    if type(node) is RawText:
        buf.append(node.content)
        return

    if node.children is None:
        return

    for child in node.children:
        _extract_raw_text_rec(child, buf)

def extract_raw_text(node: Token):
    buf = []
    _extract_raw_text_rec(node, buf)
    return buf

@app.route("/article/<name>")
def get_article(name: str):
    article = article_cache.get(name)

    if article is not None:
        last_update = article.updated_at.strftime("%Y-%m-%d")
        return render_template('article.html', article=article, last_update=last_update)
    else:
        abort(404)

@app.route("/")
def index():
    article_list = list(article_cache.articles.values())
    article_list.sort(key=lambda a: a.updated_at)
    return render_template('index.html', page_title='blog', article_list=article_list)

def main():
    global article_cache
    article_cache = ArticleCache(ARTICLE_ROOT, 10)
    article_cache.reload_articles()

    print('Serving on port 8080')
    from waitress import serve
    serve(app, port=8080)


if __name__ == "__main__": main()
