"""Exercise the actual deploy script against isolated files and fake transports."""

import hashlib
import importlib.util
import io
import json
import os
from pathlib import Path
import subprocess
import tempfile
import tarfile
import unittest


ROOT = Path(__file__).resolve().parents[2]
DEPLOY = ROOT / 'scripts/deploy/deploy-mail-ui.sh'


@unittest.skipUnless(os.name == 'posix', 'Bash deployment tests run on Linux/WSL and CI')
class DeployTests(unittest.TestCase):
    def setUp(self):
        self.assertTrue(DEPLOY.is_file(), 'Mail UI deployment has not been implemented')
        spec = importlib.util.spec_from_file_location('package_ui_deploy_test', Path(__file__).with_name('package_ui.py'))
        self.module = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(self.module)
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.home = Path(self.temp.name)
        self.runtime = self.home / 'epifera-mail'
        (self.runtime / 'incoming').mkdir(parents=True)
        (self.runtime / 'compose.yaml').write_text('name: epifera-mail\nservices: {}\n')
        (self.runtime / 'webmail.env').write_text('TEST_ONLY=fixture\n')
        self.sentinel = self.runtime / 'data/mail/do-not-touch'
        self.sentinel.parent.mkdir(parents=True)
        self.sentinel.write_text('preserved mail data')
        self.bin = self.home / 'bin'
        self.bin.mkdir()
        self.command('docker', '''#!/bin/sh
printf '%s\n' "$*" >> "$HOME/docker-calls"
''')
        self.command('sleep', '#!/bin/sh\nexit 0\n')
        self.command('curl', '''#!/usr/bin/env python3
import json,os,pathlib,sys
root=pathlib.Path.home()/'epifera-mail/ui-current'
if not root.exists():
    if sys.argv[-1]=='https://mail.epifera.com/':print('base skin');sys.exit(0)
    sys.exit(22)
release=json.loads((root/'release.json').read_text())
if os.environ.get('FAIL_RELEASE')==release['revision']:sys.exit(22)
url=sys.argv[-1]
if ('epifera.css' in url or 'brand.svg' in url) and '/static.php/' not in url:sys.exit(22)
if 'epifera.css' in url:sys.stdout.buffer.write((root/'skin/styles/epifera.css').read_bytes())
elif 'brand.svg' in url:sys.stdout.buffer.write((root/'skin/images/brand.svg').read_bytes())
else:print('skins/epifera/styles/epifera.css?v='+release['revision'])
''')
        self.env = dict(os.environ, HOME=str(self.home), PATH=str(self.bin)+':'+os.environ['PATH'], EPIFERA_MAIL_SMOKE_ATTEMPTS='2')

    def command(self, name, text):
        path = self.bin / name
        path.write_text(text)
        path.chmod(0o755)

    def release(self, revision):
        bundle = self.runtime / ('incoming/'+revision+'.tar.gz')
        self.module.build_package(ROOT, bundle, revision)
        return hashlib.sha256(bundle.read_bytes()).hexdigest()

    def run_deploy(self, revision, digest, **env):
        return subprocess.run(['bash', str(DEPLOY), revision, digest], env=dict(self.env, **env), capture_output=True, text=True)

    def test_deploy_only_recreates_webmail_and_preserves_mail(self):
        revision = 'a'*40
        result = self.run_deploy(revision, self.release(revision))
        self.assertEqual(result.returncode, 0, result.stdout+result.stderr)
        self.assertEqual((self.runtime/'ui-current').resolve().name, revision)
        self.assertEqual(self.sentinel.read_text(), 'preserved mail data')
        calls = (self.home/'docker-calls').read_text().splitlines()
        updates = [line for line in calls if ' up ' in line]
        self.assertTrue(updates)
        self.assertTrue(all(line.endswith('up -d --no-deps --force-recreate webmail') for line in updates))
        self.assertFalse(any('mailserver' in line or 'down' in line or 'prune' in line for line in calls))

    def test_failed_smoke_restores_previous_release(self):
        previous, new = 'a'*40, 'b'*40
        self.assertEqual(self.run_deploy(previous, self.release(previous)).returncode, 0)
        result = self.run_deploy(new, self.release(new), FAIL_RELEASE=new)
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual((self.runtime/'ui-current').resolve().name, previous)
        self.assertIn('restored', result.stderr)
        self.assertEqual(self.sentinel.read_text(), 'preserved mail data')

    def test_digest_mismatch_does_not_touch_live_ui(self):
        revision = 'a'*40
        self.release(revision)
        result = self.run_deploy(revision, '0'*64)
        self.assertNotEqual(result.returncode, 0)
        self.assertFalse((self.runtime/'ui-current').exists())
        self.assertFalse((self.home/'docker-calls').exists())

    def test_rejects_archive_path_escape_before_switching_ui(self):
        revision = 'c'*40
        archive = self.runtime / ('incoming/'+revision+'.tar.gz')
        with tarfile.open(archive,'w:gz') as bundle:
            member=tarfile.TarInfo('skin/../../escaped.txt')
            member.size=4
            bundle.addfile(member,io.BytesIO(b'evil'))
        result=self.run_deploy(revision,hashlib.sha256(archive.read_bytes()).hexdigest())
        self.assertNotEqual(result.returncode,0)
        self.assertFalse((self.runtime/'ui-current').exists())
        self.assertFalse((self.runtime/'escaped.txt').exists())
        self.assertFalse((self.home/'docker-calls').exists())

    def test_first_install_failure_restores_base_configuration(self):
        revision='d'*40
        result=self.run_deploy(revision,self.release(revision),FAIL_RELEASE=revision)
        self.assertNotEqual(result.returncode,0)
        self.assertFalse((self.runtime/'ui-current').exists())
        self.assertFalse((self.runtime/'compose.webmail.yaml').exists())
        self.assertIn('restored',result.stderr)
        self.assertEqual(self.sentinel.read_text(),'preserved mail data')

    def test_relative_ssh_script_path_installs_manual_rollback_helper(self):
        script=self.home/'bin/deploy-mail-ui.sh'
        script.write_bytes(DEPLOY.read_bytes())
        revision='e'*40
        result=subprocess.run(['bash','bin/deploy-mail-ui.sh',revision,self.release(revision)],cwd=self.home,env=self.env,capture_output=True,text=True)
        self.assertEqual(result.returncode,0,result.stdout+result.stderr)
        installed=self.runtime/'ops/deploy-mail-ui.sh'
        self.assertTrue(installed.is_file(),'Successful deployment must retain the rollback script')
        self.assertEqual(installed.read_bytes(),DEPLOY.read_bytes())


if __name__ == '__main__':
    unittest.main()
