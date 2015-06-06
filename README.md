# makeplural

make-plural.go translates [Unicode CLDR pluralization rules](https://github.com/unicode-cldr/cldr-core/tree/master/supplemental) to [Go](http://golang.org/) functions.
It generates the content of the "makeplural/plural" package.

This package exports only one function:
    `GetFunc(name string) (func(n float64, ordinal bool) string, error)`

## Generate "plural" package
To include any CLDR rules found in [plurals.json](https://github.com/unicode-cldr/cldr-core/blob/master/supplemental/plurals.json), just use
    `go run make-plural.go`
or to include only a subset, use
    `go run make-plural.go -culture=fr,en`

## Todo

* unit tests for generated package
* doc
