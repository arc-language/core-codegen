package main

import (
	"fmt"
	"os"

	"github.com/arc-language/core-builder/builder"
	"github.com/arc-language/core-builder/types"
	"github.com/arc-language/core-builder/ir"
	"github.com/arc-language/core-codegen/codegen" 
)

func main() {
	// 1. Initialize Builder
	b := builder.New()
	mod := b.CreateModule("fib_module")

	// 2. Define Fibonacci Function: long fib(long n)
	// Type: fn(i64) -> i64
	fibFn := b.CreateFunction("fib", types.I64, []types.Type{types.I64}, false)
	n := fibFn.Arguments[0]
	n.SetName("n")

	// Create Basic Blocks
	entry := b.CreateBlock("entry")
	recurse := b.CreateBlock("recurse")
	baseCase := b.CreateBlock("base_case")
	end := b.CreateBlock("end")

	// -- Block: Entry --
	b.SetInsertPoint(entry)
	// if n < 2 { goto base_case } else { goto recurse }
	two := b.ConstInt(types.I64, 2)
	cond := b.CreateICmpSLT(n, two, "cond")
	b.CreateCondBr(cond, baseCase, recurse)

	// -- Block: Base Case --
	b.SetInsertPoint(baseCase)
	// return n
	b.CreateBr(end)

	// -- Block: Recurse --
	b.SetInsertPoint(recurse)
	// fib(n-1)
	one := b.ConstInt(types.I64, 1)
	sub1 := b.CreateSub(n, one, "sub1")
	call1 := b.CreateCall(fibFn, []ir.Value{sub1}, "call1")

	// fib(n-2)
	sub2 := b.CreateSub(n, two, "sub2")
	call2 := b.CreateCall(fibFn, []ir.Value{sub2}, "call2")

	// result = call1 + call2
	sum := b.CreateAdd(call1, call2, "sum")
	b.CreateBr(end)

	// -- Block: End (Phi Node) --
	b.SetInsertPoint(end)
	// phi [n, base_case], [sum, recurse]
	phi := b.CreatePhi(types.I64, "result")
	phi.AddIncoming(n, baseCase)
	phi.AddIncoming(sum, recurse)
	b.CreateRet(phi)

	// 3. Print IR for verification
	fmt.Println("--- Generated IR ---")
	fmt.Println(mod.String())
	fmt.Println("--------------------")

	// 4. Generate Object File
	objBytes, err := codegen.GenerateObject(mod)
	if err != nil {
		fmt.Printf("Error generating object file: %v\n", err)
		os.Exit(1)
	}

	// 5. Write to Disk
	fileName := "sample.o"
	if err := os.WriteFile(fileName, objBytes, 0644); err != nil {
		fmt.Printf("Error writing file: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Successfully created %s (%d bytes)\n", fileName, len(objBytes))
	fmt.Println("To link and run (on Linux):")
	fmt.Println("  gcc main_stub.c sample.o -o fib_app")
}