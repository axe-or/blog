import mistletoe as mt
from mistletoe.block_token import Heading
from mistletoe.span_token import RawText
from mistletoe.token import Token

import os.path as path
from dataclasses import dataclass
from os import listdir
from flask import Flask, render_template, make_response

@dataclass
class Article:
    name : str
    display_name : str
    title : str
    contents : str

app = Flask(__name__)

ARTICLE_ROOT = 'articles'

def create_article(name: str, source: str):
    with mt.HtmlRenderer() as renderer:
        document = mt.Document(source)
        print(document.children)

def load_articles(root: str):
    filepaths = map(lambda p: path.join(root, p), filter(lambda p: p.endswith('.md'), listdir(root)))
    articles = []

    for filepath in filepaths:
        filedata = ''
        with open(filepath, 'r') as f:
            filedata = f.read()

        name, _ = path.splitext(path.basename(filepath))
        article = Article(
            name = name,
            title = f'TODO:{path.basename(filepath)}',
            contents = mt.markdown(filedata)
        )

        create_article(name, filedata)

        articles.append(article)

    return articles

@app.route("/article/<name>")
def get_article(name: str):
    articles = load_articles(ARTICLE_ROOT)
    for article in articles:
        if article.name == name:
            return render_template('article.html', article=article)
    return 'No :/'

@app.route("/")
def index():
    articles = load_articles(ARTICLE_ROOT)
    return render_template('index.html', page_title='blog', article_list=articles)

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


def create_article(name: str, source: str):
    with mt.HtmlRenderer() as renderer:
        document = mt.Document(source)
        title = name
        display_name = name

        heading = pop_first_heading(document)

        if heading is not None:
            title = renderer.render(heading)
            display_name = ' '.join(extract_raw_text(heading))

        article = Article(
            name = name,
            display_name = display_name,
            title = title,
            contents = renderer.render(document),
        )

        return article

def main():
    data = ''
    with open('articles/bar.md', 'r') as f:
        data = f.read()

    article = create_article('bar', data)
    print(article)
    # app.run(port=8080, debug=True)

if __name__ == "__main__": main()
