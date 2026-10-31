"""Offline contract checks for the deterministic public economic-policy export."""
import importlib.util
from contextlib import redirect_stderr
import io
import json
from pathlib import Path
import tempfile
import unittest
import sys

sys.dont_write_bytecode = True


class ExportTests(unittest.TestCase):
    def load_exporter(self):
        spec = importlib.util.spec_from_file_location('export_web', Path(__file__).with_name('export-web.py'))
        module = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(module)
        return module

    def test_export_is_deterministic_and_check_detects_drift_without_writing(self):
        exporter = self.load_exporter()
        with tempfile.TemporaryDirectory() as directory:
            target = Path(directory) / 'policy.json'
            self.assertEqual(exporter.export(exporter.DEFAULT_SOURCE, target, check=False), 0)
            expected = target.read_bytes()
            self.assertEqual(exporter.export(exporter.DEFAULT_SOURCE, target, check=False), 0)
            self.assertEqual(target.read_bytes(), expected)
            self.assertEqual(exporter.export(exporter.DEFAULT_SOURCE, target, check=True), 0)
            target.write_text('{}\n', encoding='utf-8')
            diagnostic = io.StringIO()
            with redirect_stderr(diagnostic):
                self.assertEqual(exporter.export(exporter.DEFAULT_SOURCE, target, check=True), 1)
            self.assertIn('differs', diagnostic.getvalue())
            self.assertEqual(target.read_text(encoding='utf-8'), '{}\n')
            policy = json.loads(expected)
            self.assertEqual(len(policy['models']), 94)
            self.assertEqual(policy['version'], 20261027)

    def test_export_rejects_unexpected_public_fields(self):
        exporter = self.load_exporter()
        source = json.loads(exporter.DEFAULT_SOURCE.read_text(encoding='utf-8'))
        source['payment_method_id'] = 'test-only-private-value'
        with tempfile.TemporaryDirectory() as directory:
            input_file = Path(directory) / 'private.json'
            input_file.write_text(json.dumps(source), encoding='utf-8')
            with self.assertRaises(ValueError):
                exporter.export(input_file, Path(directory) / 'output.json', check=False)


if __name__ == '__main__':
    unittest.main()
