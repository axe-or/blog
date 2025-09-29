import time
from locust import HttpUser, task, between

class BlogUser(HttpUser):
	wait_time = between(1, 5)
	host = 'http://localhost:8080'

	@task
	def view_article(self):
		self.client.get("/article/bar")
	@task
	def view_article(self):
		self.client.get("/article/foo")
	@task
	def view_article(self):
		self.client.get("/article/baz")

