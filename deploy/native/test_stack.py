"""Guard checks prove lifecycle commands do not target unrelated retained data."""
from argparse import Namespace
import json
import os
from pathlib import Path
from types import SimpleNamespace
import tempfile
import unittest
from unittest.mock import patch

import stack


class LifecycleGuards(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.home = Path(self.temp.name)
        self.user = SimpleNamespace(pw_uid=1000, pw_gid=1000, pw_dir=str(self.home), pw_name="fixture")
        self.lookup = patch.object(stack.pwd, "getpwnam", return_value=self.user)
        self.lookup.start()
        self.addCleanup(self.lookup.stop)

    def instance(self, root=None, confirm=None):
        return stack.Stack(Namespace(user="fixture", state_dir=str(root or self.home / "app/state"), go="go", keycloak_archive=None, confirm=confirm))

    def test_home_and_unrelated_targets_rejected(self):
        for root in (self.home, self.home / "shallow", self.home.parent / "unrelated"):
            with self.assertRaises(ValueError):
                self.instance(root)

    def test_symlink_ancestor_rejected(self):
        (self.home / "real").mkdir()
        (self.home / "link").symlink_to(self.home / "real", target_is_directory=True)
        with self.assertRaises(ValueError):
            self.instance(self.home / "link/state")

    def test_unmarked_existing_directory_is_preserved(self):
        root = self.home / "app/state"
        root.mkdir(parents=True)
        sentinel = root / "important"
        sentinel.write_text("retained")
        with self.assertRaises(FileNotFoundError):
            self.instance(root).prepare()
        self.assertEqual(sentinel.read_text(), "retained")

    def test_purge_requires_exact_confirmation_and_inactive_stack(self):
        app = self.instance()
        app.root.mkdir(parents=True)
        (app.root / "installation.json").write_text(json.dumps(app.marker))
        sentinel = app.root / "important"
        sentinel.write_text("retained")
        with self.assertRaises(ValueError):
            app.purge()
        self.assertTrue(sentinel.exists())
        app.options.confirm = str(app.root)
        with patch.object(app, "active", return_value=True), self.assertRaises(ValueError):
            app.purge()
        self.assertTrue(sentinel.exists())
        # Only this isolated temporary fixture is removed, never live Forge data.
        with patch.object(app, "active", return_value=False):
            app.purge()
        self.assertFalse(app.root.exists())

    def test_marker_mismatch_does_not_stop_services(self):
        app = self.instance()
        app.root.mkdir(parents=True)
        (app.root / "installation.json").write_text(json.dumps(dict(app.marker, uid=2000)))
        with patch.object(app, "command") as command, self.assertRaises(ValueError):
            app.stop()
        command.assert_not_called()


if __name__ == "__main__":
    unittest.main()
