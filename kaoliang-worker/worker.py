from urlparse import urlparse
import json

from celery import Celery
import requests

app = Celery('tasks',
    broker='redis://localhost:6379/0',
    backend='redis://localhost:6379/0',
)

@app.task
def send_event(url, body):
    print(json.loads(body))
    u = urlparse(url)
    if not u.scheme:
        url = 'http://' + url
    r = requests.post(url, json=json.loads(body))
    print(r.status_code)
