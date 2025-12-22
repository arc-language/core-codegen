module app

go 1.22

require (
	github.com/arc-language/core-builder v0.0.0-20251222230544-91aac0849f4f
	github.com/arc-language/core-codegen v0.0.0
)

// Point to the parent directory where the root go.mod lives
replace github.com/arc-language/core-codegen => ../