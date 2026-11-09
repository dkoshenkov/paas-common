"""Exercise trusted workflow shell against local read-only command stubs.

No GitHub request and no Git mutation: fake gh/git/go represent API and object
states. The real verifier's Git integration is covered by its separate tests.
"""
import json
import os
from pathlib import Path
import subprocess
import tempfile
import textwrap

ROOT = Path(__file__).resolve().parents[3]
WORKFLOWS = [ROOT / repo / '.github/workflows/release-pr-check.yml' for repo in ('paas-common', 'paas-ctl', 'paas-daemon')]
BASE, HEAD, OLD, PIN = 'a'*40, 'b'*40, 'c'*40, 'd'*40


def extract(text, name):
    section = text.split('      - name: ' + name + '\n', 1)[1].split('\n      - name:', 1)[0]
    code = section.split('        run: |\n', 1)[1]
    return textwrap.dedent(code)


def main():
    texts = [p.read_text() for p in WORKFLOWS]
    assert all(x == texts[0] for x in texts), 'workflows differ'
    workflow = texts[0]
    scripts = {name: extract(workflow, name) for name in (
        'Read trusted current PR and main', 'Inspect PR objects without checkout',
        'Validate trusted tools pin', 'Verify trusted tools provenance',
        'Verify release PR without executing PR contents', 'Recheck PR and main have not changed')}
    with tempfile.TemporaryDirectory() as folder:
        root = Path(folder); binpath = root/'bin'; binpath.mkdir(); (root/'release-target').mkdir(); (root/'release-tools/scripts/release-version').mkdir(parents=True)
        (root/'release-target/scripts/release-version').mkdir(parents=True)
        for directory in ('release-target', 'release-tools'):
            (root/directory/'scripts/release-version/main.go').write_text('// fixture, never compiled')
        (binpath/'gh').write_text('''#!/usr/bin/env python3
import os, pathlib, sys
if sys.argv[1:3] == ['auth','git-credential']:
 raise SystemExit('credential helper must be executed by git, not this harness')
p=pathlib.Path(os.environ['TEST_PR'] if '/pulls/' in sys.argv[-1] else os.environ['TEST_MAIN'] if '/git/ref/' in sys.argv[-1] else os.environ['TEST_REPO'])
print(p.read_text())
''')
        (binpath/'git').write_text('''#!/usr/bin/env python3
import os,sys
args=sys.argv[1:];open(os.environ['TEST_CALLS'],'a').write('git '+' '.join(args)+'\\n')
if 'update-ref' in args: raise SystemExit(0)
if 'fetch' in args:
 if 'credential.helper=!gh auth git-credential' not in args or 'GH_TOKEN' not in os.environ: raise SystemExit(9)
 raise SystemExit(0)
if 'rev-parse' in args:
 ref=args[-1]
 if ref == 'HEAD': print(os.environ['TEST_TOOLS'] if 'release-tools' in args else os.environ['TEST_BASE'])
 else: print(os.environ.get('TEST_FETCHED',os.environ['TEST_HEAD']))
 raise SystemExit(0)
if 'merge-base' in args:
 if '--is-ancestor' in args: raise SystemExit(int(os.environ.get('TEST_BAD_ANCESTRY','0')))
 print(os.environ.get('TEST_MERGE',os.environ['TEST_BASE'])); raise SystemExit(0)
if 'diff' in args:
 if os.environ.get('TEST_BAD_DIFF'): raise SystemExit(1)
 base=os.environ.get('TEST_MERGE',os.environ['TEST_BASE'])
 if base not in args: raise SystemExit('must diff from merge-base')
 sys.stdout.buffer.write(os.environ['TEST_FILES'].encode().replace(b'|',b'\\0')+b'\\0');raise SystemExit(0)
raise SystemExit('unexpected git '+repr(args))
''')
        (binpath/'go').write_text('''#!/usr/bin/env python3
import os,sys
open(os.environ['TEST_CALLS'],'a').write('go '+' '.join(sys.argv[1:])+'\\n')
if 'GH_TOKEN' in os.environ: raise SystemExit('token leaked into verifier')
if os.environ.get('TEST_FAIL_VERIFY'): raise SystemExit(1)
print('{"status":"verified"}')
''')
        for name in ('gh','git','go'): (binpath/name).chmod(0o755)
        pr = {'state':'open','base':{'sha':BASE,'ref':'main','repo':{'full_name':'dkoshenkov/paas-ctl'}},'head':{'sha':HEAD,'ref':'feature/test'},'labels':[]}
        (root/'repo.json').write_text(json.dumps({'default_branch':'main'})); (root/'pr.json').write_text(json.dumps(pr)); (root/'main.json').write_text(json.dumps({'object':{'sha':BASE}})); (root/'calls').write_text('')
        env = dict(os.environ, PATH=str(binpath)+':'+os.environ['PATH'], RUNNER_TEMP=str(root), GITHUB_WORKSPACE=str(root), GITHUB_REPOSITORY='dkoshenkov/paas-ctl', PR_NUMBER='42', PROJECT='paas-ctl', EVENT_DEFAULT_BRANCH='main', TEST_REPO=str(root/'repo.json'), EVENT_BASE_SHA=BASE, EVENT_HEAD_SHA=HEAD, BASE_SHA=BASE, HEAD_SHA=HEAD, TOOLS_SHA=PIN, RELEASE_ANCHOR='', GH_TOKEN='read-only-fixture', GITHUB_OUTPUT=str(root/'out'), TEST_PR=str(root/'pr.json'), TEST_MAIN=str(root/'main.json'), TEST_CALLS=str(root/'calls'), TEST_BASE=BASE, TEST_HEAD=HEAD, TEST_TOOLS=PIN, TEST_FILES='src/main.go')
        def run(name, changes=None, ok=True):
            e=env.copy();e.update(changes or {})
            # Only the API and fetch steps receive the short-lived token.
            if name not in ('Read trusted current PR and main','Inspect PR objects without checkout','Recheck PR and main have not changed'): e.pop('GH_TOKEN',None)
            p=subprocess.run(['bash','-euo','pipefail','-c',scripts[name]],cwd=root,env=e,text=True,capture_output=True)
            assert (p.returncode==0)==ok,(name,p.returncode,p.stdout,p.stderr)
            return p
        def snapshot():
            (root/'out').write_text('');run('Read trusted current PR and main')
        snapshot();run('Inspect PR objects without checkout',{'TEST_MERGE':OLD})
        assert 'release=false' in (root/'out').read_text(), 'stale ordinary branch misclassified'
        calls=(root/'calls').read_text();assert 'update-ref refs/heads/main '+BASE in calls and 'diff --no-ext-diff --no-renames --name-only -z '+OLD+' '+HEAD in calls
        (root/'out').write_text('');run('Inspect PR objects without checkout',{'TEST_FILES':'CHANGELOG.md|unexpected.txt'});assert 'release=true' in (root/'out').read_text()
        run('Validate trusted tools pin',{'TOOLS_SHA':''},ok=False)
        run('Validate trusted tools pin');run('Verify trusted tools provenance',{'TEST_BAD_ANCESTRY':'1'},ok=False);run('Verify trusted tools provenance')
        run('Verify release PR without executing PR contents',{'TEST_FAIL_VERIFY':'1'},ok=False)
        run('Verify release PR without executing PR contents',{'RELEASE_ANCHOR':'1.2.3@'+PIN})
        assert '--release-anchor 1.2.3@'+PIN in (root/'calls').read_text()
        run('Verify release PR without executing PR contents',{'RELEASE_ANCHOR':'malformed'},ok=False)
        run('Verify release PR without executing PR contents')
        run('Inspect PR objects without checkout',{'TEST_FETCHED':'f'*40},ok=False)
        run('Inspect PR objects without checkout',{'TEST_BAD_DIFF':'1'},ok=False)
        changed=dict(pr);changed['head']=dict(pr['head'],sha='e'*40);(root/'pr.json').write_text(json.dumps(changed))
        run('Recheck PR and main have not changed',ok=False)
        (root/'pr.json').write_text(json.dumps(pr));run('Recheck PR and main have not changed')
        (root/'repo.json').write_text(json.dumps({'default_branch':'other'}));run('Recheck PR and main have not changed',ok=False)
        (root/'repo.json').write_text(json.dumps({'default_branch':'main'}))
        bad=dict(pr);bad['base']=dict(pr['base'],sha='f'*40);(root/'pr.json').write_text(json.dumps(bad));run('Read trusted current PR and main',ok=False)
        print('PASS identical workflows; stale ordinary merge-base; detached base ref; private authenticated fetch; release intent; missing pin; bad ancestry; trusted anchor; extra-file verifier failure; moved head; diff error; final PR race; stale base; no token in verifier')


if __name__=='__main__':
    main()
