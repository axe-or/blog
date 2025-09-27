import mistletoe as mt
import os.path as path
from mistletoe.block_token import Heading
from mistletoe.span_token import RawText
from mistletoe.token import Token
from flask import Flask, render_template, make_response, abort
from dataclasses import dataclass
from os import listdir
from datetime import datetime

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

        with mt.HtmlRenderer() as renderer:
            document = mt.Document(source)
            title = name
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
    _articles : dict = {}

    @classmethod
    def reload_articles(cls, root: str):
        cls._articles = {}
        filepaths = map(lambda p: path.join(root, p), filter(lambda p: p.endswith('.md'), listdir(root)))

        for filepath in filepaths:
            filedata = ''
            with open(filepath, 'r') as f:
                filedata = f.read()

            name, _ = path.splitext(path.basename(filepath))
            article = Article(name, filedata)
            print(article)

            cls._articles[name] = article

    @classmethod
    def get(cls, name: str):
        try:
            a = cls._articles[name]
            return a
        except KeyError:
            return None

@app.route("/article/<name>")
def get_article(name: str):
    article = ArticleCache.get(name)
    if article is not None:
        last_update = article.updated_at.strftime("%Y-%m-%d")
        return render_template('article.html', article=article, last_update=last_update)
    else:
        abort(404)

@app.route("/")
def index():
    article_list = list(ArticleCache._articles.values())
    article_list.sort(key=lambda a: a.updated_at)
    return render_template('index.html', page_title='blog', article_list=article_list)

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

def pop_first_heading(doc: mt.Document) -> Heading | None:
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

def main():
    ArticleCache.reload_articles(ARTICLE_ROOT)
    app.run(port=8080, debug=True)

if __name__ == "__main__": main()
