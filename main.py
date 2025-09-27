import mistletoe as mt
import os.path as path
from dataclasses import dataclass
from os import listdir
from flask import Flask, render_template, make_response

@dataclass
class Article:
    name : str
    title : str
    contents : str

app = Flask(__name__)

ARTICLE_ROOT = 'articles'

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

        articles.append(article)

    return articles

@app.route("/article/<name>")
def get_article(name : str):
    articles = load_articles(ARTICLE_ROOT)
    for article in articles:
        if article.name == name:
            return render_template('article.html', article=article)
    return 'No :/'

@app.route("/")
def index():
    articles = load_articles(ARTICLE_ROOT)
    return render_template('index.html', page_title='blog', article_list=articles)

def main():
    app.run(port=8080, debug=True)

if __name__ == "__main__": main()
