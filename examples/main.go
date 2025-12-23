package main

import (
	"fmt"
	"os"

	"github.com/arc-language/core-builder/builder"
	"github.com/arc-language/core-builder/ir"
	"github.com/arc-language/core-builder/types"
	"github.com/arc-language/core-codegen"
)

func main() {
	// Run all examples
	examples := []struct {
		name string
		fn   func() *ir.Module
	}{
		{"hello_world", exampleHelloWorld},
		{"fibonacci", exampleFibonacci},
		{"struct_demo", exampleStructs},
		{"array_sum", exampleArraySum},
		{"control_flow", exampleControlFlow},
	}

	for _, ex := range examples {
		fmt.Printf("=== Example: %s ===\n", ex.name)

		// Build IR
		module := ex.fn()

		// Print IR
		fmt.Println("IR Output:")
		fmt.Println(module.String())
		fmt.Println()

		// Compile to object file
		objData, err := codegen.GenerateObject(module)
		if err != nil {
			fmt.Printf("Compilation failed: %v\n", err)
			continue
		}

		// Write object file
		filename := fmt.Sprintf("%s.o", ex.name)
		if err := os.WriteFile(filename, objData, 0644); err != nil {
			fmt.Printf("Failed to write file: %v\n", err)
			continue
		}

		fmt.Printf("Generated %s (%d bytes)\n", filename, len(objData))
		fmt.Println()
	}

	fmt.Println("To link and run:")
	fmt.Println("  gcc hello_world.o -o hello_world && ./hello_world")
	fmt.Println("  gcc fibonacci.o -o fibonacci && ./fibonacci")
}

// Example 1: Hello World with external printf
func exampleHelloWorld() *ir.Module {
	b := builder.New()
	m := b.CreateModule("hello_world")

	// Declare external printf: i32 printf(i8*, ...)
	printfFn := b.DeclareFunction("printf",
		types.I32,
		[]types.Type{types.NewPointer(types.I8)},
		true, // variadic
	)

	// Create global string constant
	helloStr := "Hello, World!\n\x00"
	strData := []ir.Constant{}
	for _, ch := range helloStr {
		strData = append(strData, b.ConstInt(types.I8, int64(ch)))
	}
	strArrayType := types.NewArray(types.I8, int64(len(helloStr)))
	strArray := &ir.ConstantArray{
		BaseValue: ir.BaseValue{ValType: strArrayType},
		Elements:  strData,
	}
	
	strGlobal := b.CreateGlobalConstant("hello_str", strArray)

	// Create main function: i32 main()
	mainFn := b.CreateFunction("main", types.I32, nil, false)
	entry := b.CreateBlock("entry")
	b.SetInsertPoint(entry)

	// Get pointer to string (GEP to first element)
	strPtr := b.CreateGEP(strArrayType, strGlobal, []ir.Value{
		b.ConstInt(types.I64, 0),
		b.ConstInt(types.I64, 0),
	}, "str")

	// Call printf
	b.CreateCall(printfFn, []ir.Value{strPtr}, "")

	// Return 0
	b.CreateRet(b.ConstInt(types.I32, 0))

	return m
}

// Example 2: Recursive Fibonacci
func exampleFibonacci() *ir.Module {
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
	mainFn := b.CreateFunction("main", types.I32, nil, false)
	mainEntry := b.CreateBlock("entry")
	b.SetInsertPoint(mainEntry)

	fib10 := b.CreateCall(fibFn, []ir.Value{b.ConstInt(types.I32, 10)}, "fib10")
	b.CreateRet(fib10)

	return m
}

// Example 3: Struct operations
func exampleStructs() *ir.Module {
	b := builder.New()
	m := b.CreateModule("structs")

	// Define struct Point { i32 x, i32 y }
	pointType := types.NewStruct("Point", []types.Type{
		types.I32, // x
		types.I32, // y
	}, false)
	m.Types["Point"] = pointType

	// Function: i32 distanceSquared(Point* p)
	distFn := b.CreateFunction("distanceSquared",
		types.I32,
		[]types.Type{types.NewPointer(pointType)},
		false,
	)
	distFn.Arguments[0].SetName("p")

	entry := b.CreateBlock("entry")
	b.SetInsertPoint(entry)

	pArg := distFn.Arguments[0]

	// Get x field
	xPtr := b.CreateStructGEP(pointType, pArg, 0, "x.ptr")
	x := b.CreateLoad(types.I32, xPtr, "x")

	// Get y field
	yPtr := b.CreateStructGEP(pointType, pArg, 1, "y.ptr")
	y := b.CreateLoad(types.I32, yPtr, "y")

	// Calculate x*x + y*y
	xSquared := b.CreateMul(x, x, "x_squared")
	ySquared := b.CreateMul(y, y, "y_squared")
	result := b.CreateAdd(xSquared, ySquared, "result")

	b.CreateRet(result)

	// Create main
	mainFn := b.CreateFunction("main", types.I32, nil, false)
	mainEntry := b.CreateBlock("entry")
	b.SetInsertPoint(mainEntry)

	// Allocate point on stack
	point := b.CreateAlloca(pointType, nil, "point")

	// Set x = 3
	xPtr2 := b.CreateStructGEP(pointType, point, 0, "x.ptr")
	b.CreateStore(b.ConstInt(types.I32, 3), xPtr2)

	// Set y = 4
	yPtr2 := b.CreateStructGEP(pointType, point, 1, "y.ptr")
	b.CreateStore(b.ConstInt(types.I32, 4), yPtr2)

	// Call distanceSquared(&point)
	dist := b.CreateCall(distFn, []ir.Value{point}, "dist")
	b.CreateRet(dist) // Should return 25

	return m
}

// Example 4: Array sum
func exampleArraySum() *ir.Module {
	b := builder.New()
	m := b.CreateModule("array_sum")

	// Function: i32 arraySum(i32* arr, i32 len)
	sumFn := b.CreateFunction("arraySum",
		types.I32,
		[]types.Type{
			types.NewPointer(types.I32),
			types.I32,
		},
		false,
	)
	sumFn.Arguments[0].SetName("arr")
	sumFn.Arguments[1].SetName("len")

	entry := b.CreateBlock("entry")
	loopHeader := b.CreateBlock("loop.header")
	loopBody := b.CreateBlock("loop.body")
	loopExit := b.CreateBlock("loop.exit")

	// Entry: initialize sum = 0, i = 0
	b.SetInsertPoint(entry)
	b.CreateBr(loopHeader)

	// Loop header: check i < len
	b.SetInsertPoint(loopHeader)
	iPhi := b.CreatePhi(types.I32, "i")
	sumPhi := b.CreatePhi(types.I32, "sum")
	iPhi.AddIncoming(b.ConstInt(types.I32, 0), entry)
	sumPhi.AddIncoming(b.ConstInt(types.I32, 0), entry)

	cond := b.CreateICmpSLT(iPhi, sumFn.Arguments[1], "cond")
	b.CreateCondBr(cond, loopBody, loopExit)

	// Loop body: sum += arr[i], i++
	b.SetInsertPoint(loopBody)
	arrPtr := b.CreateGEP(types.I32, sumFn.Arguments[0], []ir.Value{iPhi}, "elem.ptr")
	elem := b.CreateLoad(types.I32, arrPtr, "elem")
	newSum := b.CreateAdd(sumPhi, elem, "new_sum")
	newI := b.CreateAdd(iPhi, b.ConstInt(types.I32, 1), "new_i")
	
	iPhi.AddIncoming(newI, loopBody)
	sumPhi.AddIncoming(newSum, loopBody)
	b.CreateBr(loopHeader)

	// Loop exit: return sum
	b.SetInsertPoint(loopExit)
	b.CreateRet(sumPhi)

	// Create main
	mainFn := b.CreateFunction("main", types.I32, nil, false)
	mainEntry := b.CreateBlock("entry")
	b.SetInsertPoint(mainEntry)

	// Create array: i32[5] = {1, 2, 3, 4, 5}
	arrayType := types.NewArray(types.I32, 5)
	arr := b.CreateAlloca(arrayType, nil, "arr")

	// Initialize array elements
	for i := 0; i < 5; i++ {
		elemPtr := b.CreateGEP(arrayType, arr, []ir.Value{
			b.ConstInt(types.I64, 0),
			b.ConstInt(types.I64, int64(i)),
		}, fmt.Sprintf("arr[%d]", i))
		b.CreateStore(b.ConstInt(types.I32, int64(i+1)), elemPtr)
	}

	// Get pointer to first element
	arrPtr := b.CreateGEP(arrayType, arr, []ir.Value{
		b.ConstInt(types.I64, 0),
		b.ConstInt(types.I64, 0),
	}, "arr.ptr")

	// Call arraySum
	sum := b.CreateCall(sumFn, []ir.Value{
		arrPtr,
		b.ConstInt(types.I32, 5),
	}, "sum")

	b.CreateRet(sum) // Should return 15

	return m
}

// Example 5: Complex control flow with switch
func exampleControlFlow() *ir.Module {
	b := builder.New()
	m := b.CreateModule("control_flow")

	// Function: i32 classify(i32 n)
	classifyFn := b.CreateFunction("classify",
		types.I32,
		[]types.Type{types.I32},
		false,
	)
	classifyFn.Arguments[0].SetName("n")

	entry := b.CreateBlock("entry")
	case0 := b.CreateBlock("case0")
	case1 := b.CreateBlock("case1")
	case2 := b.CreateBlock("case2")
	defaultCase := b.CreateBlock("default")
	returnBlock := b.CreateBlock("return")

	b.SetInsertPoint(entry)
	nArg := classifyFn.Arguments[0]
	sw := b.CreateSwitch(nArg, defaultCase, 3)
	b.AddCase(sw, b.ConstInt(types.I32, 0).(*ir.ConstantInt), case0)
	b.AddCase(sw, b.ConstInt(types.I32, 1).(*ir.ConstantInt), case1)
	b.AddCase(sw, b.ConstInt(types.I32, 2).(*ir.ConstantInt), case2)

	b.SetInsertPoint(case0)
	b.CreateBr(returnBlock)

	b.SetInsertPoint(case1)
	b.CreateBr(returnBlock)

	b.SetInsertPoint(case2)
	b.CreateBr(returnBlock)

	b.SetInsertPoint(defaultCase)
	b.CreateBr(returnBlock)

	b.SetInsertPoint(returnBlock)
	phi := b.CreatePhi(types.I32, "result")
	phi.AddIncoming(b.ConstInt(types.I32, 100), case0)
	phi.AddIncoming(b.ConstInt(types.I32, 200), case1)
	phi.AddIncoming(b.ConstInt(types.I32, 300), case2)
	phi.AddIncoming(b.ConstInt(types.I32, -1), defaultCase)
	b.CreateRet(phi)

	// Create main
	mainFn := b.CreateFunction("main", types.I32, nil, false)
	mainEntry := b.CreateBlock("entry")
	b.SetInsertPoint(mainEntry)

	result := b.CreateCall(classifyFn, []ir.Value{b.ConstInt(types.I32, 1)}, "result")
	b.CreateRet(result) // Should return 200

	return m
}