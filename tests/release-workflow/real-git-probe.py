"""Run the exact workflow scripts with real Git/Go and a local GH API stub.

All repositories and commits are disposable fixtures below a temporary directory.
No GitHub network access, credential, project-repository mutation or PR checkout.
"""
import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile

from probe import WORKFLOWS, extract

ROOT = Path(__file__).resolve().parents[2]


def run(*args, cwd=None, env=None):
    result = subprocess.run(args, cwd=cwd, env=env, text=True, capture_output=True)
    if result.returncode:
        raise AssertionError((args, result.returncode, result.stdout[-2000:], result.stderr[-2000:]))
    return result.stdout.strip()


def main():
    workflow = WORKFLOWS[0].read_text()
    names = ('Read trusted current PR and main', 'Inspect PR objects without checkout',
             'Validate trusted tools pin', 'Verify trusted tools provenance',
             'Verify release PR without executing PR contents', 'Recheck PR and main have not changed')
    scripts = {name: extract(workflow, name) for name in names}
    with tempfile.TemporaryDirectory(prefix='paas-release-workflow-') as tmp:
        work = Path(tmp).resolve()
        goroot=run('go','env','GOROOT',cwd=ROOT)
        git_env = dict(os.environ, PATH=str(Path(goroot)/'bin')+':'+os.environ['PATH'], GIT_AUTHOR_NAME='Fixture', GIT_AUTHOR_EMAIL='fixture@example.test', GIT_COMMITTER_NAME='Fixture', GIT_COMMITTER_EMAIL='fixture@example.test', GOCACHE=os.environ.get('GOCACHE', str(work/'go-cache')), GOWORK='off', GOPROXY='off', GOSUMDB='off', GOTOOLCHAIN='local')
        def git(repo, *args):
            return run('git','-C',str(repo),*args,env=git_env)
        src=work/'source';src.mkdir();git(src,'init','-b','main');(src/'source.txt').write_text('initial\n')
        (src/'configs').mkdir(); (src/'configs/.release-please-manifest.json').write_text('{\".\":\"1.1.0\"}\n'); (src/'CHANGELOG.md').write_text('# Changelog\n')
        git(src,'add','.');git(src,'commit','-m','chore: previous release');anchor=git(src,'rev-parse','HEAD');git(src,'tag','v1.1.0')
        (src/'source.txt').write_text('fixed\n');git(src,'add','source.txt');git(src,'commit','-m','fix: fixture')
        base=git(src,'rev-parse','HEAD')
        tools_src=work/'tools-source';tools_src.mkdir();git(tools_src,'init','-b','main')
        (tools_src/'go.mod').write_text('module fixture.test/release-tools\n\ngo 1.27.3\n')
        shutil.copytree(ROOT/'scripts/release-version',tools_src/'scripts/release-version')
        git(tools_src,'add','.');git(tools_src,'commit','-m','test: trusted tools fixture');pin=git(tools_src,'rev-parse','HEAD')
        generated=work/'generated'
        run('go','run','./scripts/release-version','--repo',str(src),'--head',base,'--base-tag','v1.1.0','--base-sha',anchor,'--manifest','configs/.release-please-manifest.json','--output-dir',str(generated),cwd=tools_src,env=git_env)
        git(src,'switch','-c','release-fixture')
        for name in ('CHANGELOG.md','configs/.release-please-manifest.json'):
            dest=src/name;dest.parent.mkdir(parents=True,exist_ok=True);shutil.copyfile(generated/name,dest)
        git(src,'add','CHANGELOG.md','configs/.release-please-manifest.json');git(src,'commit','-m','chore: prepare release');head=git(src,'rev-parse','HEAD')
        git(src,'switch','main');git(src,'update-ref','refs/pull/42/head',head)
        run('git','clone','--bare',str(src),str(work/'remote.git'),env=git_env)
        # clone --bare copies branches but not refs/pull; add only to the fixture remote.
        git(work/'remote.git','update-ref','refs/pull/42/head',head)
        run('git','clone',str(work/'remote.git'),str(work/'release-target'),env=git_env)
        git(work/'release-target','checkout','--detach',base);git(work/'release-target','branch','-D','main')
        assert subprocess.run(['git','-C',str(work/'release-target'),'symbolic-ref','-q','HEAD'],capture_output=True).returncode != 0
        run('git','clone',str(tools_src),str(work/'release-tools'),env=git_env)
        git(work/'release-tools','checkout','--detach',pin)
        pr={'state':'open','base':{'sha':base,'ref':'main','repo':{'full_name':'dkoshenkov/paas-ctl'}},'head':{'sha':head,'ref':'release/fixture'},'labels':[]}
        (work/'repo.json').write_text(json.dumps({'default_branch':'main'}));(work/'pr.json').write_text(json.dumps(pr));(work/'main.json').write_text(json.dumps({'object':{'sha':base}}))
        bindir=work/'bin';bindir.mkdir();gh=bindir/'gh'
        gh.write_text('#!/usr/bin/env python3\nimport os,pathlib,sys\np=pathlib.Path(os.environ["TEST_PR"] if "/pulls/" in sys.argv[-1] else os.environ["TEST_MAIN"] if "/git/ref/" in sys.argv[-1] else os.environ["TEST_REPO"])\nprint(p.read_text())\n');gh.chmod(0o755)
        env=dict(git_env,PATH=str(bindir)+':'+git_env['PATH'],RUNNER_TEMP=str(work),GITHUB_WORKSPACE=str(work),GITHUB_REPOSITORY='dkoshenkov/paas-ctl',PR_NUMBER='42',PROJECT='paas-ctl',EVENT_DEFAULT_BRANCH='main',TEST_REPO=str(work/'repo.json'),EVENT_BASE_SHA=base,EVENT_HEAD_SHA=head,BASE_SHA=base,HEAD_SHA=head,TOOLS_SHA=pin,RELEASE_ANCHOR='1.1.0@'+anchor,GH_TOKEN='fixture-only',GITHUB_OUTPUT=str(work/'out'),TEST_PR=str(work/'pr.json'),TEST_MAIN=str(work/'main.json'))
        (work/'out').write_text('')
        for name in names:
            e=env.copy()
            if name not in ('Read trusted current PR and main','Inspect PR objects without checkout','Recheck PR and main have not changed'): e.pop('GH_TOKEN',None)
            out=run('bash','-euo','pipefail','-c',scripts[name],cwd=work,env=e)
            if name=='Verify release PR without executing PR contents':
                result=json.loads(out);assert result['status']=='verified' and result['version']=='1.1.1' and result['pr_head']==head
        assert 'release=true' in (work/'out').read_text()
        assert git(work/'release-target','rev-parse','HEAD')==base
        assert git(work/'release-target','rev-parse','refs/heads/main')==base
        assert (work/'release-target/CHANGELOG.md').read_text() == '# Changelog\n', 'untrusted PR tree was checked out'
        # A release candidate with an unexpected file must reach the real
        # verifier and fail, without checking out either PR tree.
        git(src,'switch','release-fixture');(src/'unexpected.txt').write_text('untrusted\n')
        git(src,'add','unexpected.txt');git(src,'commit','--amend','--no-edit');bad_head=git(src,'rev-parse','HEAD')
        git(work/'remote.git','fetch',str(src),bad_head+':refs/pull/42/head','--force')
        pr['head']['sha']=bad_head;(work/'pr.json').write_text(json.dumps(pr))
        env['EVENT_HEAD_SHA']=bad_head;env['HEAD_SHA']=bad_head;(work/'out').write_text('')
        for name in ('Read trusted current PR and main','Inspect PR objects without checkout'):
            run('bash','-euo','pipefail','-c',scripts[name],cwd=work,env=env)
        assert 'release=true' in (work/'out').read_text()
        verify_env=env.copy();verify_env.pop('GH_TOKEN',None)
        failed=subprocess.run(['bash','-euo','pipefail','-c',scripts['Verify release PR without executing PR contents']],cwd=work,env=verify_env,text=True,capture_output=True)
        assert failed.returncode != 0, 'unexpected extra file passed real verifier'
        assert not (work/'release-target/unexpected.txt').exists()
        print('PASS exact shell steps, real detached Git, PR-object fetch, pinned tools and Go verifier, final API recheck; extra file rejected')


if __name__=='__main__':
    main()
