#!/usr/bin/env python3
"""cut_changelog.py 单测：python .github/scripts/cut_changelog_test.py"""

from __future__ import annotations

import tempfile
import unittest
from pathlib import Path

import cut_changelog as cc


SAMPLE = """# Changelog

## [Unreleased]

### 新增

- feature A

## [1.0.0] - 2026-01-01

- old
"""


class CutChangelogTest(unittest.TestCase):
    def test_plan_unreleased(self) -> None:
        _, sections = cc.parse(SAMPLE)
        self.assertEqual(cc.plan_release(sections, set(), "1.1.0"), "1.1.0")
        self.assertEqual(cc.plan_release(sections, {"1.1.0"}, "1.1.0"), "")

    def test_notes_only_keeps_changelog(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            changelog = root / "CHANGELOG.md"
            notes = root / "release-notes.md"
            changelog.write_text(SAMPLE, encoding="utf-8", newline="\n")
            # emulate apply --notes-only
            preamble, sections = cc.parse(changelog.read_text(encoding="utf-8"))
            updated = cc.promote(sections, "1.1.0", "2026-10-16")
            notes.write_text(cc.notes_for(updated, "1.1.0"), encoding="utf-8", newline="\n")
            self.assertIn("feature A", notes.read_text(encoding="utf-8"))
            self.assertEqual(changelog.read_text(encoding="utf-8"), SAMPLE)
            self.assertIn("Unreleased", changelog.read_text(encoding="utf-8"))

    def test_promote_writes_version_section(self) -> None:
        preamble, sections = cc.parse(SAMPLE)
        updated = cc.promote(sections, "1.1.0", "2026-10-16")
        text = cc.render(preamble, updated)
        self.assertIn("## [1.1.0] - 2026-10-16", text)
        self.assertIn("feature A", text)


if __name__ == "__main__":
    unittest.main()
