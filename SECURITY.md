# Security policy

## Report a vulnerability

Do not report a suspected vulnerability in a public issue.
Use the [private vulnerability reporting](https://github.com/OpenRAE/go-rae-archcheck/security/advisories/new) of this repository.
If it is not available, email **security@autarchy.ai** with `go-rae-archcheck security report` in the subject.

Include enough detail to reproduce and assess the issue:

- the affected package, command or document
- the affected version, commit or branch
- the steps that reproduce it
- the expected and the actual behavior
- the impact
- a proof of concept or logs, if available

## Scope

Reports are most useful for these parts of the checker:

- the parsing and validation of models and design records
- the loading of Go packages
- the command, its output and its exit statuses
- the repository automation that handles untrusted input

## What to expect

The project has one maintainer and no response time commitment.
The maintainer reviews reports as time allows.
Do not publish the details of an exploit before the maintainer has had reasonable time to assess the report and prepare a fix.
