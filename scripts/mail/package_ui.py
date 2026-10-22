"""Build the public mail UI artifact; runtime data and credentials are never read."""

import argparse
import gzip
import hashlib
import io
import json
from pathlib import Path
import re
import tarfile


TOKEN = b'__EPIFERA_UI_REVISION__'
STATIC_SUFFIXES = {'.css', '.html', '.json', '.svg', '.png', '.jpg', '.webp', '.woff2', '.js'}
TEXT_SUFFIXES = {'.css', '.html', '.json', '.svg', '.js', '.php', '.yaml'}


def build_package(root: Path, output: Path, revision: str):
    if not re.fullmatch(r'[0-9a-f]{40}', revision):
        raise ValueError('Expected a 40-character source revision')
    root = root.resolve()
    source = root / 'deployments/mail'
    theme = source / 'skins/epifera'
    if theme.is_symlink() or not theme.is_dir():
        raise ValueError('Expected a real theme directory')
    files = {}

    def add(name, path):
        if path.is_symlink() or not path.is_file():
            raise ValueError('Package sources must be regular files: ' + name)
        content = path.read_bytes()
        if path.suffix in TEXT_SUFFIXES:
            content = content.replace(b'\r\n', b'\n').replace(TOKEN, revision.encode('ascii'))
        files[name] = content

    for path in sorted(theme.rglob('*')):
        if path.is_symlink():
            raise ValueError('Symlinks are not allowed in the skin')
        if not path.is_file():
            continue
        if path.suffix not in STATIC_SUFFIXES or any(part.startswith('.') for part in path.relative_to(theme).parts):
            raise ValueError('Unexpected skin file: ' + path.name)
        add('skin/' + path.relative_to(theme).as_posix(), path)
    required = {'skin/meta.json', 'skin/styles/epifera.css', 'skin/templates/includes/layout.html'}
    if not required <= files.keys():
        raise ValueError('Incomplete skin')
    if json.loads(files['skin/meta.json']).get('extends') != 'elastic':
        raise ValueError('The mail skin must extend Elastic')

    brand_path = root / 'web/platform/branding/brand.json'
    brand = json.loads(brand_path.read_text(encoding='utf-8-sig'))
    if not isinstance(brand.get('name'), str) or not brand['name'].strip():
        raise ValueError('Missing canonical brand name')
    public = (root / 'web/platform/public').resolve()
    for key, destination in ((brand['logoPath'], 'skin/images/brand.svg'),
                             (brand['icons']['shortcut'], 'skin/images/favicon.png')):
        if not isinstance(key, str) or not key.startswith('/'):
            raise ValueError('Brand assets must use public-relative paths')
        asset = public / key.lstrip('/')
        try:
            asset.resolve().relative_to(public)
        except ValueError as exc:
            raise ValueError('Brand asset escapes the public directory') from exc
        add(destination, asset)
    add('config/brand.json', brand_path)
    add('config/epifera.inc.php', source / 'roundcube.config.php')
    add('compose.webmail.yaml', source / 'compose.webmail.yaml')
    manifest = {'revision': revision, 'files': {
        name: hashlib.sha256(content).hexdigest() for name, content in sorted(files.items())
    }}
    files['release.json'] = (json.dumps(manifest, sort_keys=True, indent=2) + '\n').encode()
    output.parent.mkdir(parents=True, exist_ok=True)
    with output.open('wb') as raw, gzip.GzipFile(fileobj=raw, mode='wb', mtime=0, filename='') as zipped:
        with tarfile.open(fileobj=zipped, mode='w') as archive:
            for name, content in sorted(files.items()):
                entry = tarfile.TarInfo(name)
                entry.size = len(content)
                entry.mode = 0o644
                archive.addfile(entry, io.BytesIO(content))
    return manifest


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--revision', required=True)
    parser.add_argument('--output', type=Path, required=True)
    args = parser.parse_args()
    build_package(Path(__file__).resolve().parents[2], args.output, args.revision)
    print('Public mail UI package ready for revision ' + args.revision)
