# go-rae-archcheck

go-rae-archcheck checks a Go module against one architecture model.
The model states the layers of the module and the packages that each layer owns.
It also states the layers and external modules that each layer can use, and rules for standard-library imports and symbols.

The checker also requires a design record in the same change as each structural change to the model.

OpenRÆ repositories and the products that build on them pin a release of this checker.
The repository holds only generic code and generic test fixtures.

## Use

Install a pinned release, then run the checker from the module root:

```sh
go install github.com/OpenRAE/go-rae-archcheck/cmd/archcheck@VERSION
archcheck
```

[`docs/model.md`](docs/model.md) describes the model format, the checks and the command flags.
This repository checks itself with its own model, `architecture/model.yaml`.

## Status

The first release is in development.
[Issue 3](https://github.com/OpenRAE/go-rae-archcheck/issues/3) lists the standard that the checker meets before that release.

## Contribute

[`CONTRIBUTING.md`](CONTRIBUTING.md) describes the setup, the checks and the standards.
Report a vulnerability as [`SECURITY.md`](SECURITY.md) states, and follow the [code of conduct](CODE_OF_CONDUCT.md).

## License

MIT, as `LICENSE` states.
