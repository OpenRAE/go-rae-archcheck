"""Validate the shared, repository-neutral parts of a pull request body."""

import argparse
import json
import re
from pathlib import Path


RELATED_ISSUES = "Related Issues"
REQUIRED_SECTIONS = ("Summary", RELATED_ISSUES, "ADR Impact", "Test Plan")
HEADING = re.compile(r"^## ([^\n]+)\s*$", re.MULTILINE)


def check_pr_body(body: str, author: str) -> list[str]:
    """Return clear failures for missing or unfilled PR sections."""
    if author == "dependabot[bot]":
        return []

    headings = list(HEADING.finditer(body))
    sections = {}
    for index, heading in enumerate(headings):
        end = headings[index + 1].start() if index + 1 < len(headings) else len(body)
        content = re.sub(r"<!--.*?-->", "", body[heading.end():end], flags=re.DOTALL)
        sections[heading.group(1).strip()] = content.strip()

    failures = []
    for name in REQUIRED_SECTIONS:
        content = sections.get(name, "")
        if not content or content in {"-", "- [ ]"}:
            failures.append(f"Fill in ## {name}")

    related_issues = sections.get(RELATED_ISSUES, "")
    if related_issues and related_issues != "None" and not re.search(r"#\d+\b", related_issues):
        failures.append(f"## {RELATED_ISSUES} must link an issue number or say None")
    return failures


def main() -> None:
    """Read GitHub's PR event and fail CI for an incomplete body."""
    parser = argparse.ArgumentParser()
    parser.add_argument("--event", type=Path, required=True)
    args = parser.parse_args()
    event = json.loads(args.event.read_text(encoding="utf-8"))
    pull_request = event["pull_request"]
    failures = check_pr_body(pull_request.get("body") or "", pull_request["user"]["login"])
    if failures:
        parser.exit(1, "\n".join(failures) + "\n")


if __name__ == "__main__":
    main()
