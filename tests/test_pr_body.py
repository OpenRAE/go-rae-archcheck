"""PR body checks protect required context while allowing dependency updates."""

import io
import json
import sys
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch

from scripts.check_pr_body import check_pr_body, main


VALID_BODY = """## Summary
Bootstraps repository checks.

## Related Issues
Refs #123

## ADR Impact
No ADR required.

## Test Plan
make check passed.
"""


class PullRequestBodyTest(unittest.TestCase):
    """Exercise a completed body and the two common incomplete cases."""

    def test_complete_body_passes(self):
        self.assertEqual(check_pr_body(VALID_BODY, "contributor"), [])

    def test_no_linked_issue_passes(self):
        self.assertEqual(check_pr_body(VALID_BODY.replace("Refs #123", "None"), "contributor"), [])

    def test_empty_template_fails(self):
        body = "\n".join(f"## {name}\n<!-- Fill in -->" for name in ("Summary", "Related Issues", "ADR Impact", "Test Plan"))
        self.assertIn("Fill in ## Summary", check_pr_body(body, "contributor"))

    def test_issue_link_is_required(self):
        self.assertIn("## Related Issues must link an issue number or say None", check_pr_body(VALID_BODY.replace("Refs #123", "See issue"), "contributor"))

    def test_issue_link_in_comment_does_not_count(self):
        body = VALID_BODY.replace("Refs #123", "<!-- Refs #123 -->\nSee issue")
        self.assertIn("## Related Issues must link an issue number or say None", check_pr_body(body, "contributor"))

    def test_dependabot_body_uses_dependency_workflow(self):
        self.assertEqual(check_pr_body("", "dependabot[bot]"), [])

    def test_event_cli_checks_the_pr_body(self):
        with tempfile.TemporaryDirectory() as directory:
            event = Path(directory) / "event.json"
            event.write_text(json.dumps({"pull_request": {"body": VALID_BODY, "user": {"login": "contributor"}}}))
            with patch.object(sys, "argv", ["check_pr_body.py", "--event", str(event)]):
                main()

    def test_event_cli_rejects_missing_sections(self):
        with tempfile.TemporaryDirectory() as directory:
            event = Path(directory) / "event.json"
            event.write_text(json.dumps({"pull_request": {"body": "", "user": {"login": "contributor"}}}))
            with patch.object(sys, "argv", ["check_pr_body.py", "--event", str(event)]):
                with patch.object(sys, "stderr", io.StringIO()):
                    with self.assertRaises(SystemExit) as result:
                        main()
            self.assertEqual(result.exception.code, 1)
