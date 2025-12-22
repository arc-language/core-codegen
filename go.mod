module example

go 1.22

require (
    github.com/arc-language/core-builder v0.0.0
    github.com/arc-language/core-codegen v0.0.0
)

// THIS LINE FIXES THE ERROR:
replace github.com/arc-language/core-codegen => ../core-codegen