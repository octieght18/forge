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

    def test_dispatcher_secret_upgrade_retains_existing_values(self):
        app = self.instance()
        app.root.mkdir(parents=True)
        names = ("postgres", "migrator", "runtime", "identity", "admin", "ahmad", "second-owner", "operator")
        before = {name: "a" * 48 for name in names}
        private = app.root / "secrets.json"
        private.write_text(json.dumps(before))
        cursor = app.root / "cursor.key"
        cursor.write_text("retained synthetic cursor")
        with patch.object(stack.os, "fchown"):
            app.prepare_secrets()
        after = json.loads(private.read_text())
        self.assertEqual({name: after[name] for name in names}, before)
        self.assertRegex(after["dispatcher"], r"^[0-9a-f]{48}$")
        self.assertEqual(private.stat().st_mode & 0o777, 0o600)
        self.assertEqual(cursor.read_text(), "retained synthetic cursor")
        with patch.object(app, "write_secrets") as write:
            app.prepare_secrets()
        write.assert_not_called()

    def test_malformed_old_secrets_are_not_upgraded(self):
        app = self.instance()
        app.root.mkdir(parents=True)
        private = app.root / "secrets.json"
        private.write_text('{"runtime":"malformed"}')
        with patch.object(app, "write_secrets") as write, self.assertRaises(ValueError):
            app.prepare_secrets()
        write.assert_not_called()

    def test_failed_secret_replacement_preserves_old_configuration(self):
        app = self.instance()
        app.root.mkdir(parents=True)
        names = ("postgres", "migrator", "runtime", "identity", "admin", "ahmad", "second-owner", "operator")
        private = app.root / "secrets.json"
        original = json.dumps({name: "a" * 48 for name in names})
        private.write_text(original)
        with patch.object(stack.os, "fchown"), patch.object(stack.os, "replace", side_effect=OSError("injected replacement failure")), self.assertRaises(OSError):
            app.prepare_secrets()
        self.assertEqual(private.read_text(), original)
        self.assertEqual(list(app.root.glob(".secrets-*")), [])


if __name__ == "__main__":
    unittest.main()
