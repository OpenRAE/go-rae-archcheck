# go-rae-archcheck

go-rae-archcheck checks a Go module against one architecture model.
The model states the layers of the module, the packages that each layer owns, the layers and external modules that each layer can use, and rules for standard-library imports and symbols.

The checker also requires a design record in the same change as each structural change to the model.

OpenRÆ repositories and the products that build on them pin a release of this checker.
The repository holds only generic code and generic test fixtures.

## Status

The first release is in development.

## License

MIT, as `LICENSE` states.
