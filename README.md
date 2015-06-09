# makeplural

make-plural.go translates [Unicode CLDR pluralization rules](https://github.com/unicode-cldr/cldr-core/tree/master/supplemental) to [Go](http://golang.org/) functions.
It generates the content of the "makeplural/plural" package.

This package exports only one function:

    `GetFunc(name string) (func(n interface{}, ordinal bool) string, error)`

## Update "plural" package
To include any CLDR rules found in [plurals.json](https://github.com/unicode-cldr/cldr-core/blob/master/supplemental/plurals.json), just use
    `go run make-plural.go`
or to include only a subset, use
    `go run make-plural.go -culture=fr,en`

then you should run the unit tests to ensure everything went well :

`
    cd plural
    go test
`

## Warning about float values
Depending on the country, you should consider providing float values as string or its specific rules may not be successfully applied.

example :
`
    fn, _ := GetFunc("sl")
    named_key := fn(x, false)
`
with `x := 0.0`, named_key will holds "other" while "few" is expected, but if `x := "0.0"` everything will be ok!

## Todo

* doc
