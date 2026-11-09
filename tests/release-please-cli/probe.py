"""Release Please 17.11.2 against an in-process GitHub API fixture only.

Run: copy package.json and package-lock.json into a temporary directory,
     npm ci --prefix that directory, then set RELEASE_PLEASE_CLI to its
     node_modules/.bin/release-please and run this script.
No GitHub token or network service is used. Expectations include known failures.
"""
import base64
import hashlib
import json
import os
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
import subprocess
import threading
from urllib.parse import parse_qs, unquote, urlparse

ROOT = Path(__file__).resolve().parent
CLI = Path(os.environ.get('RELEASE_PLEASE_CLI', str(ROOT / 'node_modules' / '.bin' / 'release-please')))
PAGE = {'hasNextPage': False, 'endCursor': None}
BOUNDARY, HEAD = 'b' * 40, 'a' * 40


class Fixture:
    def __init__(self, message, sections=(), bootstrap=False, missing_boundary=False):
        self.message, self.sections, self.bootstrap = message, sections, bootstrap
        self.missing_boundary = missing_boundary
        config = {'packages': {'.': {'release-type': 'go'}}}
        if sections:
            config['changelog-sections'] = [{'type': s, 'section': s.title(), 'hidden': False} for s in sections]
        if bootstrap:
            config['bootstrap-sha'] = BOUNDARY
        self.files = {'configs/release-please-config.json': json.dumps(config),
                      'configs/.release-please-manifest.json': json.dumps({} if bootstrap else {'.': '1.1.0'}),
                      'CHANGELOG.md': '# Changelog\n', 'go.mod': 'module example.test/paas\n\ngo 1.24\n'}
        self.requests = []
        self.pr = None
        self.branch = False
        self.trees = []

    def handler(self):
        fixture = self

        class Handler(BaseHTTPRequestHandler):
            def log_message(self, *args):
                pass

            def respond(self, data, code=200):
                raw = json.dumps(data).encode()
                self.send_response(code)
                self.send_header('Content-Type', 'application/json')
                self.send_header('Content-Length', str(len(raw)))
                self.end_headers()
                self.wfile.write(raw)

            def do_GET(self):
                path = unquote(urlparse(self.path).path)
                fixture.requests.append(('GET', path))
                if path == '/repos/local/test':
                    return self.respond({'default_branch': 'main', 'name': 'test', 'full_name': 'local/test', 'owner': {'login': 'local'}})
                if path.startswith('/repos/local/test/git/trees/'):
                    return self.respond({'tree': [{'path': k, 'type': 'blob', 'mode': '100644', 'sha': hashlib.sha1(k.encode()).hexdigest()} for k in fixture.files], 'truncated': False})
                if path.startswith('/repos/local/test/git/blobs/'):
                    sha = path.rsplit('/', 1)[1]
                    for name, data in fixture.files.items():
                        if hashlib.sha1(name.encode()).hexdigest() == sha:
                            return self.respond({'sha': sha, 'encoding': 'base64', 'content': base64.b64encode(data.encode()).decode()})
                if path == '/repos/local/test/branches/main':
                    return self.respond({'commit': {'sha': HEAD}})
                if path.startswith('/repos/local/test/git/ref/heads/'):
                    return self.respond({'ref':'refs/heads/release-please--branches--main'}) if fixture.branch else self.respond({'message':'Not Found'},404)
                if path.startswith('/repos/local/test/git/commits/'):
                    return self.respond({'sha':HEAD,'tree':{'sha':'d'*40}})
                if path == '/repos/local/test/pulls':
                    params=parse_qs(urlparse(self.path).query)
                    return self.respond([fixture.pr] if fixture.pr and params.get('state',['open'])[0]=='open' else [])
                if path == '/repos/local/test/pulls/1' and fixture.pr:
                    return self.respond(fixture.pr)
                if path == '/repos/local/test/tags':
                    return self.respond([])
                if path.startswith('/repos/local/test/commits/'):
                    return self.respond({'files': [{'filename': 'src/main.go', 'status': 'modified'}]})
                return self.respond({'message': 'Not Found'}, 404)

            def do_PATCH(self):
                path=unquote(urlparse(self.path).path)
                body=json.loads(self.rfile.read(int(self.headers['Content-Length'])))
                fixture.requests.append(('PATCH',path))
                if path.startswith('/repos/local/test/git/refs/heads/'):
                    return self.respond({'ref':path,'object':{'sha':body['sha']}})
                return self.respond({'message':'Unexpected patch'},400)

            def do_POST(self):
                body = json.loads(self.rfile.read(int(self.headers['Content-Length'])))
                query = body.get('query', '')
                path=urlparse(self.path).path
                if path == '/repos/local/test/git/refs':
                    fixture.branch=True;fixture.requests.append(('POST',path));return self.respond({'url':'http://fixture/ref','ref':body['ref']},201)
                if path == '/repos/local/test/git/trees':
                    fixture.trees.append(body);fixture.requests.append(('POST',path));return self.respond({'sha':'e'*40},201)
                if path == '/repos/local/test/git/commits':
                    fixture.requests.append(('POST',path));return self.respond({'sha':'f'*40,'url':'http://fixture/commit'},201)
                if path == '/repos/local/test/pulls':
                    fixture.requests.append(('POST',path));fixture.pr={'number':1,'title':body['title'],'body':body['body'],'head':{'ref':body['head'].split(':',1)[-1],'label':body['head']},'base':{'ref':body['base']},'labels':[],'url':'http://fixture/pr/1'};return self.respond(fixture.pr,201)
                if path == '/repos/local/test/issues/1/labels':
                    fixture.requests.append(('POST',path));fixture.pr['labels']=[{'name':x} for x in body['labels']];return self.respond(fixture.pr['labels'])
                fixture.requests.append(('POST', urlparse(self.path).path, query.split('(', 1)[0]))
                if 'pullRequestsSince' in query:
                    def commit(sha, message):
                        return {'sha': sha, 'message': message, 'associatedPullRequests': {'nodes': []}, 'author': {'name': 'Fixture', 'email': 'fixture@example.test', 'user': {'login': 'fixture'}}}
                    nodes = [commit(HEAD, fixture.message)]
                    if not fixture.missing_boundary:
                        nodes.append(commit(BOUNDARY, 'chore: prior release'))
                    nodes.append(commit('c' * 40, 'feat: historical excluded'))
                    return self.respond({'data': {'repository': {'ref': {'target': {'history': {'nodes': nodes, 'pageInfo': PAGE}}}}}})
                if 'query releases' in query:
                    nodes = [] if fixture.bootstrap else [{'name': 'v1.1.0', 'tag': {'name': 'v1.1.0'}, 'tagCommit': {'oid': BOUNDARY}, 'url': 'https://example.test/v1.1.0', 'description': 'old', 'isDraft': False}]
                    return self.respond({'data': {'repository': {'releases': {'nodes': nodes, 'pageInfo': PAGE}}}})
                if 'mergedPullRequests' in query:
                    return self.respond({'data': {'repository': {'pullRequests': {'nodes': [], 'pageInfo': PAGE}}}})
                return self.respond({'message': 'Unexpected query'}, 400)

        return Handler

    def run(self, version, dry_run=True, repeat=False):
        server = ThreadingHTTPServer(('127.0.0.1', 0), self.handler())
        thread = threading.Thread(target=server.serve_forever, daemon=True)
        thread.start()
        base = 'http://127.0.0.1:' + str(server.server_port)
        try:
            cmd = [str(CLI), 'release-pr', '--repo-url', 'local/test', '--api-url', base, '--graphql-url', base,
                   '--target-branch', 'main', '--token', 'fixture-only', '--config-file', 'configs/release-please-config.json',
                   '--manifest-file', 'configs/.release-please-manifest.json', '--release-as', version, '--trace']
            if dry_run: cmd.append('--dry-run')
            p = subprocess.run(cmd, text=True, capture_output=True, timeout=45)
            assert p.returncode == 0, p.stdout[-2500:] + p.stderr[-2500:]
            assert all(entry[1].startswith(('/repos/local/test', '/graphql')) for entry in self.requests), self.requests
            output = p.stdout + p.stderr
            if repeat:
                p = subprocess.run(cmd, text=True, capture_output=True, timeout=45)
                assert p.returncode == 0, p.stdout[-2500:] + p.stderr[-2500:]
                output += "\n=== RETRY ===\n" + p.stdout + p.stderr
            return output
        finally:
            server.shutdown()
            server.server_close()
            thread.join()


def main():
    if not CLI.exists():
        raise SystemExit('Set RELEASE_PLEASE_CLI to the pinned binary from npm ci')
    actual = subprocess.check_output([str(CLI), '--version'], text=True).strip()
    assert actual == '17.11.2', actual
    print('CLI', actual, flush=True)
    scenarios = [
        ('fix', 'fix: repair', (), False, '1.1.1', 1),
        ('chore hidden by default', 'chore: maintenance', (), False, '1.2.0', 0),
        ('chore explicitly visible', 'chore: maintenance', ('feat', 'fix', 'chore'), False, '1.2.0', 1),
        ('unknown type hidden', 'other: change', ('feat', 'fix', 'chore'), False, '1.2.0', 0),
        ('unknown type explicit', 'other: change', ('feat', 'fix', 'chore', 'other'), False, '1.2.0', 1),
        ('nonconventional', 'update dependency metadata', ('feat', 'fix', 'chore', 'other'), False, '1.2.0', 0),
        ('bootstrap', 'fix: initial', (), True, '1.0.0', 1),
    ]
    for label, message, sections, bootstrap, version, count in scenarios:
        out = Fixture(message, sections, bootstrap).run(version)
        assert f'Would open {count} pull requests' in out, (label, out[-3000:])
        if count:
            assert 'release ' + version in out and f'".":"{version}"' in out, (label, out[-3000:])
        assert 'historical excluded' not in out, (label,out[-3000:])
        print('PASS', label, 'PRs', count, flush=True)
    missing = Fixture('fix: repair', missing_boundary=True)
    missing_output = missing.run('1.1.1')
    assert 'Would open 1 pull requests' in missing_output and 'historical excluded' in missing_output
    print('PASS missing boundary: CLI proposes PR including unbounded history', flush=True)
    print('missing boundary warning:', 'not found' if 'not found' in missing_output.lower() else 'none explicit', flush=True)
    live = Fixture('fix: repair')
    output = live.run('1.1.1', dry_run=False, repeat=True)
    assert live.pr and live.pr['title'].endswith('1.1.1'), output[-3000:]
    created = [r for r in live.requests if r == ('POST','/repos/local/test/pulls')]
    assert len(created) == 1 and 'remained the same' in output, (live.requests,output[-3000:])
    assert len(live.trees) == 1, live.trees
    manifest = next(x for x in live.trees[0]['tree'] if x['path']=='configs/.release-please-manifest.json')
    assert json.loads(manifest['content'])['.']=='1.1.1'
    print('PASS create + retry: one mock PR, same 1.1.1 manifest, no second tree', flush=True)


if __name__ == '__main__':
    main()
