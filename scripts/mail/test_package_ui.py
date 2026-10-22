import importlib.util
import json
from pathlib import Path
import tarfile
import tempfile
import unittest


MODULE_PATH = Path(__file__).with_name('package_ui.py')


class PackageTests(unittest.TestCase):
    def setUp(self):
        self.assertTrue(MODULE_PATH.is_file(), 'Mail UI packager has not been implemented')
        spec = importlib.util.spec_from_file_location('mail_package_ui', MODULE_PATH)
        self.module = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(self.module)
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.write('deployments/mail/skins/epifera/meta.json', '{"extends":"elastic"}')
        self.write('deployments/mail/skins/epifera/styles/epifera.css', 'body { color: red; }')
        self.write('deployments/mail/skins/epifera/templates/includes/layout.html', '__EPIFERA_UI_REVISION__')
        self.write('deployments/mail/roundcube.config.php', '<?php // __EPIFERA_UI_REVISION__')
        self.write('deployments/mail/compose.webmail.yaml', 'services: {}')
        self.write('web/platform/branding/brand.json', json.dumps({
            'name': 'Example Brand', 'logoPath': '/assets/mark.svg',
            'icons': {'shortcut': '/assets/icon.png'},
        }))
        self.write('web/platform/public/assets/mark.svg', '<svg xmlns="http://www.w3.org/2000/svg"/>')
        self.write('web/platform/public/assets/icon.png', 'test image')
        self.revision = 'a' * 40

    def write(self, name, content):
        path = self.root / name
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(content, encoding='utf-8')
        return path

    def build(self):
        out = self.root / 'out/ui.tar.gz'
        self.module.build_package(self.root, out, self.revision)
        return out

    def test_package_uses_shared_brand_and_omits_runtime_credentials(self):
        self.write('deployments/mail/webmail.env', 'PRIVATE=must-not-ship')
        self.write('deployments/mail/data/mail/private.txt', 'private message')
        archive = self.build()
        with tarfile.open(archive) as bundle:
            names = bundle.getnames()
            self.assertNotIn('webmail.env', names)
            self.assertFalse(any('private' in name for name in names))
            brand = json.load(bundle.extractfile('config/brand.json'))
            self.assertEqual(brand['name'], 'Example Brand')
            self.assertIn(self.revision.encode(), bundle.extractfile('skin/templates/includes/layout.html').read())
            self.assertEqual(json.load(bundle.extractfile('release.json'))['revision'], self.revision)
            self.assertIn('skin/images/brand.svg', names)
            self.assertIn('skin/images/favicon.png', names)
        self.assertEqual(archive.read_bytes(), self.build().read_bytes())

    def test_rejects_brand_path_escape(self):
        self.write('web/platform/branding/brand.json', json.dumps({
            'name': 'Bad', 'logoPath': '/../private.svg',
            'icons': {'shortcut': '/assets/icon.png'},
        }))
        with self.assertRaises(ValueError):
            self.build()

    def test_rejects_private_files_inside_skin(self):
        self.write('deployments/mail/skins/epifera/account.pem', 'PRIVATE KEY')
        with self.assertRaises(ValueError):
            self.build()

    def test_rejects_symlinked_theme_assets(self):
        target = self.write('outside.css', 'private')
        link = self.root / 'deployments/mail/skins/epifera/styles/external.css'
        try:
            link.symlink_to(target)
        except OSError:
            self.skipTest('Symlink creation not available; Linux CI covers this case')
        with self.assertRaises(ValueError):
            self.build()


if __name__ == '__main__':
    unittest.main()
