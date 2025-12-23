package main

import (
	"fmt"
	"os"

	"github.com/arc-language/core-builder/builder"
	"github.com/arc-language/core-builder/ir"
	"github.com/arc-language/core-builder/types"
	"github.com/arc-language/core-codegen/codegen"
)

func main() {
	fmt.Println("=== Fibonacci Example ===")

	// Create IR builder
	b := builder.New()
	m := b.CreateModule("fibonacci")

	// Create function: i32 fibonacci(i32 n)
	fibFn := b.CreateFunction("fibonacci",
		types.I32,
		[]types.Type{types.I32},
		false,
	)
	fibFn.Arguments[0].SetName("n")

	entry := b.CreateBlock("entry")
	baseCase := b.CreateBlock("base_case")
	recursive := b.CreateBlock("recursive")
	returnBlock := b.CreateBlock("return")

	// Entry block: check if n <= 1
	b.SetInsertPoint(entry)
	nArg := fibFn.Arguments[0]
	isBaseCase := b.CreateICmpSLE(nArg, b.ConstInt(types.I32, 1), "is_base")
	b.CreateCondBr(isBaseCase, baseCase, recursive)

	// Base case: return n
	b.SetInsertPoint(baseCase)
	b.CreateBr(returnBlock)

	// Recursive case: fib(n-1) + fib(n-2)
	b.SetInsertPoint(recursive)
	nMinus1 := b.CreateSub(nArg, b.ConstInt(types.I32, 1), "n_minus_1")
	nMinus2 := b.CreateSub(nArg, b.ConstInt(types.I32, 2), "n_minus_2")

	fib1 := b.CreateCall(fibFn, []ir.Value{nMinus1}, "fib1")
	fib2 := b.CreateCall(fibFn, []ir.Value{nMinus2}, "fib2")
	result := b.CreateAdd(fib1, fib2, "result")
	b.CreateBr(returnBlock)

	// Return block with phi
	b.SetInsertPoint(returnBlock)
	phi := b.CreatePhi(types.I32, "retval")
	phi.AddIncoming(nArg, baseCase)
	phi.AddIncoming(result, recursive)
	b.CreateRet(phi)

	// Create main to test: main() { return fibonacci(10); }
	b.CreateFunction("main", types.I32, nil, false)
	mainEntry := b.CreateBlock("entry")
	b.SetInsertPoint(mainEntry)

	fib10 := b.CreateCall(fibFn, []ir.Value{b.ConstInt(types.I32, 10)}, "fib10")
	b.CreateRet(fib10)

	// Print IR
	fmt.Println("\nGenerated IR:")
	fmt.Println(m.String())

	// Compile to object file
	fmt.Println("\nCompiling...")
	objData, err := codegen.GenerateObject(m)
	if err != nil {
		fmt.Printf("Compilation failed: %v\n", err)
		os.Exit(1)
	}

	// Write object file
	filename := "fibonacci.o"
	if err := os.WriteFile(filename, objData, 0644); err != nil {
		fmt.Printf("Failed to write file: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\n✓ Generated %s (%d bytes)\n", filename, len(objData))
	fmt.Println("\nTo link and run:")
	fmt.Println("  gcc fibonacci.o -o fibonacci && ./fibonacci")
	fmt.Println("  echo $?  # Should print 55 (fibonacci of 10)")
}