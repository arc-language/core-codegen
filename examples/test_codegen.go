package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/arc-language/core-builder/builder"
	"github.com/arc-language/core-builder/ir"
	"github.com/arc-language/core-builder/types"
	"github.com/arc-language/core-codegen/codegen"
)

type TestCase struct {
	Name           string
	BuildFunc      func(*builder.Builder) *ir.Module
	ExpectedOutput int
}

func main() {
	tests := []TestCase{
		{
			Name:           "simple_return",
			BuildFunc:      buildSimpleReturn,
			ExpectedOutput: 42,
		},
		{
			Name:           "addition",
			BuildFunc:      buildAddition,
			ExpectedOutput: 15,
		},
		{
			Name:           "subtraction",
			BuildFunc:      buildSubtraction,
			ExpectedOutput: 5,
		},
		{
			Name:           "multiplication",
			BuildFunc:      buildMultiplication,
			ExpectedOutput: 24,
		},
		{
			Name:           "division",
			BuildFunc:      buildDivision,
			ExpectedOutput: 5,
		},
		{
			Name:           "modulo",
			BuildFunc:      buildModulo,
			ExpectedOutput: 3,
		},
		{
			Name:           "comparison_eq",
			BuildFunc:      buildComparisonEq,
			ExpectedOutput: 1,
		},
		{
			Name:           "comparison_ne",
			BuildFunc:      buildComparisonNe,
			ExpectedOutput: 1,
		},
		{
			Name:           "comparison_lt",
			BuildFunc:      buildComparisonLt,
			ExpectedOutput: 1,
		},
		{
			Name:           "comparison_le",
			BuildFunc:      buildComparisonLe,
			ExpectedOutput: 1,
		},
		{
			Name:           "comparison_gt",
			BuildFunc:      buildComparisonGt,
			ExpectedOutput: 0,
		},
		{
			Name:           "comparison_ge",
			BuildFunc:      buildComparisonGe,
			ExpectedOutput: 1,
		},
		{
			Name:           "all_comparison_operators",
			BuildFunc:      buildAllComparisons,
			ExpectedOutput: 6,
		},
		{
			Name:           "if_then_else",
			BuildFunc:      buildIfThenElse,
			ExpectedOutput: 10,
		},
		{
			Name:           "nested_if",
			BuildFunc:      buildNestedIf,
			ExpectedOutput: 30,
		},
		{
			Name:           "simple_loop",
			BuildFunc:      buildSimpleLoop,
			ExpectedOutput: 10,
		},
		{
			Name:           "nested_loops",
			BuildFunc:      buildNestedLoops,
			ExpectedOutput: 55,
		},
		{
			Name:           "factorial",
			BuildFunc:      buildFactorial,
			ExpectedOutput: 120, // 5!
		},
		{
			Name:           "fibonacci",
			BuildFunc:      buildFibonacci,
			ExpectedOutput: 55, // fib(10)
		},
		{
			Name:           "bitwise_and",
			BuildFunc:      buildBitwiseAnd,
			ExpectedOutput: 8,
		},
		{
			Name:           "bitwise_or",
			BuildFunc:      buildBitwiseOr,
			ExpectedOutput: 15,
		},
		{
			Name:           "bitwise_xor",
			BuildFunc:      buildBitwiseXor,
			ExpectedOutput: 7,
		},
		{
			Name:           "shift_left",
			BuildFunc:      buildShiftLeft,
			ExpectedOutput: 32,
		},
		{
			Name:           "shift_right",
			BuildFunc:      buildShiftRight,
			ExpectedOutput: 2,
		},
		{
			Name:           "negative_numbers",
			BuildFunc:      buildNegativeNumbers,
			ExpectedOutput: 253,
		},
		{
			Name:           "zero_division_check",
			BuildFunc:      buildZeroDivisionCheck,
			ExpectedOutput: 10,
		},
		{
			Name:           "complex_expression",
			BuildFunc:      buildComplexExpression,
			ExpectedOutput: 42,
		},
		{
			Name:           "multiple_args",
			BuildFunc:      buildMultipleArgs,
			ExpectedOutput: 42,
		},
		{
			Name:           "nested_calls",
			BuildFunc:      buildNestedCalls,
			ExpectedOutput: 17,
		},
		{
			Name:           "select_instruction",
			BuildFunc:      buildSelect,
			ExpectedOutput: 100,
		},
		{
			Name:           "switch_statement",
			BuildFunc:      buildSwitchStatement,
			ExpectedOutput: 30,
		},
		{
			Name:           "memory_alloca_load_store",
			BuildFunc:      buildMemoryOps,
			ExpectedOutput: 99,
		},
		{
			Name:           "pointer_arithmetic",
			BuildFunc:      buildPointerArithmetic,
			ExpectedOutput: 15,
		},
		{
			Name:           "struct_operations",
			BuildFunc:      buildStructOps,
			ExpectedOutput: 42,
		},
		{
			Name:           "array_operations",
			BuildFunc:      buildArrayOps,
			ExpectedOutput: 10,
		},
		{
			Name:           "casting_operations",
			BuildFunc:      buildCastingOps,
			ExpectedOutput: 42,
		},
		{
			Name:           "phi_with_multiple_preds",
			BuildFunc:      buildComplexPhi,
			ExpectedOutput: 15,
		},
		{
			Name:           "early_return",
			BuildFunc:      buildEarlyReturn,
			ExpectedOutput: 5,
		},
		{
			Name:           "max_function",
			BuildFunc:      buildMaxFunction,
			ExpectedOutput: 88,
		},
	}

	passed := 0
	failed := 0

	fmt.Println("=== Running Codegen Tests ===\n")

	for _, test := range tests {
		fmt.Printf("Running: %-30s ... ", test.Name)
		
		if runTest(test) {
			fmt.Println("✓ PASS")
			passed++
		} else {
			fmt.Println("✗ FAIL")
			failed++
		}
	}

	fmt.Printf("\n=== Results ===\n")
	fmt.Printf("Passed: %d/%d\n", passed, len(tests))
	fmt.Printf("Failed: %d/%d\n", failed, len(tests))
	
	if failed > 0 {
		os.Exit(1)
	}
}

func runTest(test TestCase) bool {
	// Build IR
	b := builder.New()
	m := test.BuildFunc(b)

	// Compile to object file
	objData, err := codegen.GenerateObject(m)
	if err != nil {
		fmt.Printf("\n  Compilation error: %v", err)
		return false
	}

	// Write object file
	tmpDir := os.TempDir()
	objPath := filepath.Join(tmpDir, test.Name+".o")
	exePath := filepath.Join(tmpDir, test.Name)

	if err := os.WriteFile(objPath, objData, 0644); err != nil {
		fmt.Printf("\n  Write error: %v", err)
		return false
	}
	
	// Don't defer removal yet - we may need to dump it on failure
	success := true
	deferredCleanup := func() {
		os.Remove(objPath)
		os.Remove(exePath)
	}

	// Link with gcc
	cmd := exec.Command("gcc", objPath, "-o", exePath)
	if output, err := cmd.CombinedOutput(); err != nil {
		fmt.Printf("\n  Link error: %v\n%s", err, output)
		dumpObjectFile(objPath)
		deferredCleanup()
		return false
	}

	// Run the executable
	cmd = exec.Command(exePath)
	if err := cmd.Run(); err != nil {
		// Check if it's an exit code error
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode := exitErr.ExitCode()
			if exitCode != test.ExpectedOutput {
				fmt.Printf("\n  Expected exit code %d, got %d", test.ExpectedOutput, exitCode)
				success = false
			}
		} else {
			fmt.Printf("\n  Runtime error: %v", err)
			success = false
		}
		
		if !success {
			dumpObjectFile(objPath)
			deferredCleanup()
			return false
		}
		deferredCleanup()
		return true
	}

	// Exit code 0
	if test.ExpectedOutput != 0 {
		fmt.Printf("\n  Expected exit code %d, got 0", test.ExpectedOutput)
		dumpObjectFile(objPath)
		deferredCleanup()
		return false
	}

	deferredCleanup()
	return true
}

func dumpObjectFile(objPath string) {
	fmt.Printf("\n  === Object file dump ===\n")
	cmd := exec.Command("objdump", "-x", "-d", "-r", objPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("  Failed to dump object file: %v\n", err)
		return
	}
	fmt.Printf("%s\n", output)
}

// ============================================================================
// Test IR Builders
// ============================================================================

func buildSimpleReturn(b *builder.Builder) *ir.Module {
	m := b.CreateModule("simple_return")
	
	b.CreateFunction("main", types.I32, nil, false)
	entry := b.CreateBlock("entry")
	b.SetInsertPoint(entry)
	b.CreateRet(b.ConstInt(types.I32, 42))
	
	return m
}

func buildAddition(b *builder.Builder) *ir.Module {
	m := b.CreateModule("addition")
	
	b.CreateFunction("main", types.I32, nil, false)
	entry := b.CreateBlock("entry")
	b.SetInsertPoint(entry)
	
	a := b.ConstInt(types.I32, 7)
	b2 := b.ConstInt(types.I32, 8)
	result := b.CreateAdd(a, b2, "result")
	b.CreateRet(result)
	
	return m
}

func buildSubtraction(b *builder.Builder) *ir.Module {
	m := b.CreateModule("subtraction")
	
	b.CreateFunction("main", types.I32, nil, false)
	entry := b.CreateBlock("entry")
	b.SetInsertPoint(entry)
	
	a := b.ConstInt(types.I32, 12)
	b2 := b.ConstInt(types.I32, 7)
	result := b.CreateSub(a, b2, "result")
	b.CreateRet(result)
	
	return m
}

func buildMultiplication(b *builder.Builder) *ir.Module {
	m := b.CreateModule("multiplication")
	
	b.CreateFunction("main", types.I32, nil, false)
	entry := b.CreateBlock("entry")
	b.SetInsertPoint(entry)
	
	a := b.ConstInt(types.I32, 6)
	b2 := b.ConstInt(types.I32, 4)
	result := b.CreateMul(a, b2, "result")
	b.CreateRet(result)
	
	return m
}

func buildDivision(b *builder.Builder) *ir.Module {
	m := b.CreateModule("division")
	
	b.CreateFunction("main", types.I32, nil, false)
	entry := b.CreateBlock("entry")
	b.SetInsertPoint(entry)
	
	a := b.ConstInt(types.I32, 25)
	b2 := b.ConstInt(types.I32, 5)
	result := b.CreateSDiv(a, b2, "result")
	b.CreateRet(result)
	
	return m
}

func buildModulo(b *builder.Builder) *ir.Module {
	m := b.CreateModule("modulo")
	
	b.CreateFunction("main", types.I32, nil, false)
	entry := b.CreateBlock("entry")
	b.SetInsertPoint(entry)
	
	a := b.ConstInt(types.I32, 23)
	b2 := b.ConstInt(types.I32, 5)
	result := b.CreateSRem(a, b2, "result")
	b.CreateRet(result)
	
	return m
}

func buildComparisonEq(b *builder.Builder) *ir.Module {
	m := b.CreateModule("comparison_eq")
	
	b.CreateFunction("main", types.I32, nil, false)
	entry := b.CreateBlock("entry")
	b.SetInsertPoint(entry)
	
	a := b.ConstInt(types.I32, 5)
	b2 := b.ConstInt(types.I32, 5)
	cmp := b.CreateICmpEQ(a, b2, "cmp")
	result := b.CreateZExt(cmp, types.I32, "result")
	b.CreateRet(result)
	
	return m
}

func buildComparisonNe(b *builder.Builder) *ir.Module {
	m := b.CreateModule("comparison_ne")
	
	b.CreateFunction("main", types.I32, nil, false)
	entry := b.CreateBlock("entry")
	b.SetInsertPoint(entry)
	
	a := b.ConstInt(types.I32, 5)
	b2 := b.ConstInt(types.I32, 7)
	cmp := b.CreateICmpNE(a, b2, "cmp")
	result := b.CreateZExt(cmp, types.I32, "result")
	b.CreateRet(result)
	
	return m
}

func buildComparisonLt(b *builder.Builder) *ir.Module {
	m := b.CreateModule("comparison_lt")
	
	b.CreateFunction("main", types.I32, nil, false)
	entry := b.CreateBlock("entry")
	b.SetInsertPoint(entry)
	
	a := b.ConstInt(types.I32, 3)
	b2 := b.ConstInt(types.I32, 7)
	cmp := b.CreateICmpSLT(a, b2, "cmp")
	result := b.CreateZExt(cmp, types.I32, "result")
	b.CreateRet(result)
	
	return m
}

func buildComparisonLe(b *builder.Builder) *ir.Module {
	m := b.CreateModule("comparison_le")
	
	b.CreateFunction("main", types.I32, nil, false)
	entry := b.CreateBlock("entry")
	b.SetInsertPoint(entry)
	
	a := b.ConstInt(types.I32, 5)
	b2 := b.ConstInt(types.I32, 5)
	cmp := b.CreateICmpSLE(a, b2, "cmp")
	result := b.CreateZExt(cmp, types.I32, "result")
	b.CreateRet(result)
	
	return m
}

func buildComparisonGt(b *builder.Builder) *ir.Module {
	m := b.CreateModule("comparison_gt")
	
	b.CreateFunction("main", types.I32, nil, false)
	entry := b.CreateBlock("entry")
	b.SetInsertPoint(entry)
	
	a := b.ConstInt(types.I32, 3)
	b2 := b.ConstInt(types.I32, 7)
	cmp := b.CreateICmpSGT(a, b2, "cmp")
	result := b.CreateZExt(cmp, types.I32, "result")
	b.CreateRet(result)
	
	return m
}

func buildComparisonGe(b *builder.Builder) *ir.Module {
	m := b.CreateModule("comparison_ge")
	
	b.CreateFunction("main", types.I32, nil, false)
	entry := b.CreateBlock("entry")
	b.SetInsertPoint(entry)
	
	a := b.ConstInt(types.I32, 7)
	b2 := b.ConstInt(types.I32, 7)
	cmp := b.CreateICmpSGE(a, b2, "cmp")
	result := b.CreateZExt(cmp, types.I32, "result")
	b.CreateRet(result)
	
	return m
}

func buildAllComparisons(b *builder.Builder) *ir.Module {
	m := b.CreateModule("all_comparisons")
	
	b.CreateFunction("main", types.I32, nil, false)
	entry := b.CreateBlock("entry")
	b.SetInsertPoint(entry)
	
	// Test all 6 major comparison operators and count trues
	a := b.ConstInt(types.I32, 5)
	b2 := b.ConstInt(types.I32, 3)
	
	eq := b.CreateICmpEQ(a, a, "eq")    // true
	ne := b.CreateICmpNE(a, b2, "ne")   // true
	gt := b.CreateICmpSGT(a, b2, "gt")  // true
	ge := b.CreateICmpSGE(a, a, "ge")   // true
	lt := b.CreateICmpSLT(b2, a, "lt")  // true
	le := b.CreateICmpSLE(b2, a, "le")  // true
	
	// Extend all to i32
	eq32 := b.CreateZExt(eq, types.I32, "eq32")
	ne32 := b.CreateZExt(ne, types.I32, "ne32")
	gt32 := b.CreateZExt(gt, types.I32, "gt32")
	ge32 := b.CreateZExt(ge, types.I32, "ge32")
	lt32 := b.CreateZExt(lt, types.I32, "lt32")
	le32 := b.CreateZExt(le, types.I32, "le32")
	
	// Sum them all
	sum1 := b.CreateAdd(eq32, ne32, "sum1")
	sum2 := b.CreateAdd(sum1, gt32, "sum2")
	sum3 := b.CreateAdd(sum2, ge32, "sum3")
	sum4 := b.CreateAdd(sum3, lt32, "sum4")
	result := b.CreateAdd(sum4, le32, "result")
	
	b.CreateRet(result)
	
	return m
}

func buildIfThenElse(b *builder.Builder) *ir.Module {
	m := b.CreateModule("if_then_else")
	
	b.CreateFunction("main", types.I32, nil, false)
	entry := b.CreateBlock("entry")
	thenBlock := b.CreateBlock("then")
	elseBlock := b.CreateBlock("else")
	merge := b.CreateBlock("merge")
	
	b.SetInsertPoint(entry)
	cond := b.CreateICmpSGT(b.ConstInt(types.I32, 5), b.ConstInt(types.I32, 3), "cond")
	b.CreateCondBr(cond, thenBlock, elseBlock)
	
	b.SetInsertPoint(thenBlock)
	thenVal := b.ConstInt(types.I32, 10)
	b.CreateBr(merge)
	
	b.SetInsertPoint(elseBlock)
	elseVal := b.ConstInt(types.I32, 20)
	b.CreateBr(merge)
	
	b.SetInsertPoint(merge)
	phi := b.CreatePhi(types.I32, "result")
	phi.AddIncoming(thenVal, thenBlock)
	phi.AddIncoming(elseVal, elseBlock)
	b.CreateRet(phi)
	
	return m
}

func buildNestedIf(b *builder.Builder) *ir.Module {
	m := b.CreateModule("nested_if")
	
	b.CreateFunction("main", types.I32, nil, false)
	entry := b.CreateBlock("entry")
	outer_then := b.CreateBlock("outer_then")
	inner_then := b.CreateBlock("inner_then")
	inner_else := b.CreateBlock("inner_else")
	inner_merge := b.CreateBlock("inner_merge")
	outer_else := b.CreateBlock("outer_else")
	final_merge := b.CreateBlock("final_merge")
	
	b.SetInsertPoint(entry)
	cond1 := b.CreateICmpSGT(b.ConstInt(types.I32, 10), b.ConstInt(types.I32, 5), "cond1")
	b.CreateCondBr(cond1, outer_then, outer_else)
	
	b.SetInsertPoint(outer_then)
	cond2 := b.CreateICmpSLT(b.ConstInt(types.I32, 3), b.ConstInt(types.I32, 7), "cond2")
	b.CreateCondBr(cond2, inner_then, inner_else)
	
	b.SetInsertPoint(inner_then)
	val1 := b.ConstInt(types.I32, 30)
	b.CreateBr(inner_merge)
	
	b.SetInsertPoint(inner_else)
	val2 := b.ConstInt(types.I32, 40)
	b.CreateBr(inner_merge)
	
	b.SetInsertPoint(inner_merge)
	phi1 := b.CreatePhi(types.I32, "inner_result")
	phi1.AddIncoming(val1, inner_then)
	phi1.AddIncoming(val2, inner_else)
	b.CreateBr(final_merge)
	
	b.SetInsertPoint(outer_else)
	val3 := b.ConstInt(types.I32, 50)
	b.CreateBr(final_merge)
	
	b.SetInsertPoint(final_merge)
	phi2 := b.CreatePhi(types.I32, "result")
	phi2.AddIncoming(phi1, inner_merge)
	phi2.AddIncoming(val3, outer_else)
	b.CreateRet(phi2)
	
	return m
}

func buildSimpleLoop(b *builder.Builder) *ir.Module {
	m := b.CreateModule("simple_loop")
	
	b.CreateFunction("main", types.I32, nil, false)
	entry := b.CreateBlock("entry")
	loop := b.CreateBlock("loop")
	exit := b.CreateBlock("exit")
	
	b.SetInsertPoint(entry)
	b.CreateBr(loop)
	
	b.SetInsertPoint(loop)
	i := b.CreatePhi(types.I32, "i")
	i.AddIncoming(b.ConstInt(types.I32, 0), entry)
	
	next := b.CreateAdd(i, b.ConstInt(types.I32, 1), "next")
	i.AddIncoming(next, loop)
	
	cond := b.CreateICmpSLT(next, b.ConstInt(types.I32, 10), "cond")
	b.CreateCondBr(cond, loop, exit)
	
	b.SetInsertPoint(exit)
	b.CreateRet(next)
	
	return m
}

func buildNestedLoops(b *builder.Builder) *ir.Module {
	m := b.CreateModule("nested_loops")
	
	b.CreateFunction("main", types.I32, nil, false)
	entry := b.CreateBlock("entry")
	outerLoop := b.CreateBlock("outer_loop")
	innerLoop := b.CreateBlock("inner_loop")
	innerExit := b.CreateBlock("inner_exit")
	outerExit := b.CreateBlock("outer_exit")
	
	b.SetInsertPoint(entry)
	b.CreateBr(outerLoop)
	
	// Outer loop: i from 1 to 4
	b.SetInsertPoint(outerLoop)
	i := b.CreatePhi(types.I32, "i")
	sum := b.CreatePhi(types.I32, "sum")
	i.AddIncoming(b.ConstInt(types.I32, 1), entry)
	sum.AddIncoming(b.ConstInt(types.I32, 0), entry)
	
	b.CreateBr(innerLoop)
	
	// Inner loop: j from i to i (just once for simplicity, but demonstrates nesting)
	b.SetInsertPoint(innerLoop)
	j := b.CreatePhi(types.I32, "j")
	innerSum := b.CreatePhi(types.I32, "inner_sum")
	j.AddIncoming(i, outerLoop)
	innerSum.AddIncoming(sum, outerLoop)
	
	newSum := b.CreateAdd(innerSum, j, "new_sum")
	nextJ := b.CreateAdd(j, b.ConstInt(types.I32, 1), "next_j")
	
	j.AddIncoming(nextJ, innerLoop)
	innerSum.AddIncoming(newSum, innerLoop)
	
	innerCond := b.CreateICmpSLE(nextJ, i, "inner_cond")
	b.CreateCondBr(innerCond, innerLoop, innerExit)
	
	b.SetInsertPoint(innerExit)
	nextI := b.CreateAdd(i, b.ConstInt(types.I32, 1), "next_i")
	i.AddIncoming(nextI, innerExit)
	sum.AddIncoming(newSum, innerExit)
	
	outerCond := b.CreateICmpSLE(nextI, b.ConstInt(types.I32, 10), "outer_cond")
	b.CreateCondBr(outerCond, outerLoop, outerExit)
	
	b.SetInsertPoint(outerExit)
	b.CreateRet(newSum)
	
	return m
}

func buildFactorial(b *builder.Builder) *ir.Module {
	m := b.CreateModule("factorial")
	
	// factorial function
	factFn := b.CreateFunction("factorial", types.I32, []types.Type{types.I32}, false)
	factFn.Arguments[0].SetName("n")
	
	entry := b.CreateBlock("entry")
	baseCase := b.CreateBlock("base_case")
	recursive := b.CreateBlock("recursive")
	ret := b.CreateBlock("return")
	
	b.SetInsertPoint(entry)
	n := factFn.Arguments[0]
	isBase := b.CreateICmpSLE(n, b.ConstInt(types.I32, 1), "is_base")
	b.CreateCondBr(isBase, baseCase, recursive)
	
	b.SetInsertPoint(baseCase)
	b.CreateBr(ret)
	
	b.SetInsertPoint(recursive)
	nMinus1 := b.CreateSub(n, b.ConstInt(types.I32, 1), "n_minus_1")
	factNMinus1 := b.CreateCall(factFn, []ir.Value{nMinus1}, "fact_n_minus_1")
	result := b.CreateMul(n, factNMinus1, "result")
	b.CreateBr(ret)
	
	b.SetInsertPoint(ret)
	phi := b.CreatePhi(types.I32, "retval")
	phi.AddIncoming(b.ConstInt(types.I32, 1), baseCase)
	phi.AddIncoming(result, recursive)
	b.CreateRet(phi)
	
	// main function
	b.CreateFunction("main", types.I32, nil, false)
	mainEntry := b.CreateBlock("entry")
	b.SetInsertPoint(mainEntry)
	
	fact5 := b.CreateCall(factFn, []ir.Value{b.ConstInt(types.I32, 5)}, "fact5")
	b.CreateRet(fact5)
	
	return m
}

func buildFibonacci(b *builder.Builder) *ir.Module {
	m := b.CreateModule("fibonacci")
	
	fibFn := b.CreateFunction("fibonacci", types.I32, []types.Type{types.I32}, false)
	fibFn.Arguments[0].SetName("n")
	
	entry := b.CreateBlock("entry")
	baseCase := b.CreateBlock("base_case")
	recursive := b.CreateBlock("recursive")
	ret := b.CreateBlock("return")
	
	b.SetInsertPoint(entry)
	n := fibFn.Arguments[0]
	isBase := b.CreateICmpSLE(n, b.ConstInt(types.I32, 1), "is_base")
	b.CreateCondBr(isBase, baseCase, recursive)
	
	b.SetInsertPoint(baseCase)
	b.CreateBr(ret)
	
	b.SetInsertPoint(recursive)
	nMinus1 := b.CreateSub(n, b.ConstInt(types.I32, 1), "n_minus_1")
	nMinus2 := b.CreateSub(n, b.ConstInt(types.I32, 2), "n_minus_2")
	fib1 := b.CreateCall(fibFn, []ir.Value{nMinus1}, "fib1")
	fib2 := b.CreateCall(fibFn, []ir.Value{nMinus2}, "fib2")
	result := b.CreateAdd(fib1, fib2, "result")
	b.CreateBr(ret)
	
	b.SetInsertPoint(ret)
	phi := b.CreatePhi(types.I32, "retval")
	phi.AddIncoming(n, baseCase)
	phi.AddIncoming(result, recursive)
	b.CreateRet(phi)
	
	b.CreateFunction("main", types.I32, nil, false)
	mainEntry := b.CreateBlock("entry")
	b.SetInsertPoint(mainEntry)
	fib10 := b.CreateCall(fibFn, []ir.Value{b.ConstInt(types.I32, 10)}, "fib10")
	b.CreateRet(fib10)
	
	return m
}

func buildBitwiseAnd(b *builder.Builder) *ir.Module {
	m := b.CreateModule("bitwise_and")
	
	b.CreateFunction("main", types.I32, nil, false)
	entry := b.CreateBlock("entry")
	b.SetInsertPoint(entry)
	
	a := b.ConstInt(types.I32, 12) // 1100
	b2 := b.ConstInt(types.I32, 10) // 1010
	result := b.CreateAnd(a, b2, "result") // 1000 = 8
	b.CreateRet(result)
	
	return m
}

func buildBitwiseOr(b *builder.Builder) *ir.Module {
	m := b.CreateModule("bitwise_or")
	
	b.CreateFunction("main", types.I32, nil, false)
	entry := b.CreateBlock("entry")
	b.SetInsertPoint(entry)
	
	a := b.ConstInt(types.I32, 12) // 1100
	b2 := b.ConstInt(types.I32, 3)  // 0011
	result := b.CreateOr(a, b2, "result") // 1111 = 15
	b.CreateRet(result)
	
	return m
}

func buildBitwiseXor(b *builder.Builder) *ir.Module {
	m := b.CreateModule("bitwise_xor")
	
	b.CreateFunction("main", types.I32, nil, false)
	entry := b.CreateBlock("entry")
	b.SetInsertPoint(entry)
	
	a := b.ConstInt(types.I32, 12) // 1100
	b2 := b.ConstInt(types.I32, 11) // 1011
	result := b.CreateXor(a, b2, "result") // 0111 = 7
	b.CreateRet(result)
	
	return m
}

func buildShiftLeft(b *builder.Builder) *ir.Module {
	m := b.CreateModule("shift_left")
	
	b.CreateFunction("main", types.I32, nil, false)
	entry := b.CreateBlock("entry")
	b.SetInsertPoint(entry)
	
	a := b.ConstInt(types.I32, 4)
	shift := b.ConstInt(types.I32, 3)
	result := b.CreateShl(a, shift, "result") // 4 << 3 = 32
	b.CreateRet(result)
	
	return m
}

func buildShiftRight(b *builder.Builder) *ir.Module {
	m := b.CreateModule("shift_right")
	
	b.CreateFunction("main", types.I32, nil, false)
	entry := b.CreateBlock("entry")
	b.SetInsertPoint(entry)
	
	a := b.ConstInt(types.I32, 16)
	shift := b.ConstInt(types.I32, 3)
	result := b.CreateLShr(a, shift, "result") // 16 >> 3 = 2
	b.CreateRet(result)
	
	return m
}

func buildNegativeNumbers(b *builder.Builder) *ir.Module {
	m := b.CreateModule("negative_numbers")
	
	b.CreateFunction("main", types.I32, nil, false)
	entry := b.CreateBlock("entry")
	b.SetInsertPoint(entry)
	
	// -3 & 0xFF should give 253 (two's complement)
	neg := b.ConstInt(types.I32, -3)
	mask := b.ConstInt(types.I32, 0xFF)
	result := b.CreateAnd(neg, mask, "result")
	b.CreateRet(result)
	
	return m
}

func buildZeroDivisionCheck(b *builder.Builder) *ir.Module {
	m := b.CreateModule("zero_division_check")
	
	b.CreateFunction("main", types.I32, nil, false)
	entry := b.CreateBlock("entry")
	checkZero := b.CreateBlock("check_zero")
	divBlock := b.CreateBlock("divide")
	noDiv := b.CreateBlock("no_divide")
	merge := b.CreateBlock("merge")
	
	b.SetInsertPoint(entry)
	divisor := b.ConstInt(types.I32, 5)
	b.CreateBr(checkZero)
	
	b.SetInsertPoint(checkZero)
	isZero := b.CreateICmpEQ(divisor, b.ConstInt(types.I32, 0), "is_zero")
	b.CreateCondBr(isZero, noDiv, divBlock)
	
	b.SetInsertPoint(divBlock)
	divResult := b.CreateSDiv(b.ConstInt(types.I32, 50), divisor, "div_result")
	b.CreateBr(merge)
	
	b.SetInsertPoint(noDiv)
	defaultVal := b.ConstInt(types.I32, 0)
	b.CreateBr(merge)
	
	b.SetInsertPoint(merge)
	phi := b.CreatePhi(types.I32, "result")
	phi.AddIncoming(divResult, divBlock)
	phi.AddIncoming(defaultVal, noDiv)
	b.CreateRet(phi)
	
	return m
}

func buildComplexExpression(b *builder.Builder) *ir.Module {
	m := b.CreateModule("complex_expression")
	
	b.CreateFunction("main", types.I32, nil, false)
	entry := b.CreateBlock("entry")
	b.SetInsertPoint(entry)
	
	// (6 * 7) + (12 / 4) - 3 = 42 + 3 - 3 = 42
	a := b.CreateMul(b.ConstInt(types.I32, 6), b.ConstInt(types.I32, 7), "mul")
	b2 := b.CreateSDiv(b.ConstInt(types.I32, 12), b.ConstInt(types.I32, 4), "div")
	c := b.CreateAdd(a, b2, "add")
	result := b.CreateSub(c, b.ConstInt(types.I32, 3), "result")
	b.CreateRet(result)
	
	return m
}

func buildMultipleArgs(b *builder.Builder) *ir.Module {
	m := b.CreateModule("multiple_args")
	
	// create function: sum(a, b, c, d, e, f, g) = a + b + c + d + e + f + g
	sumFn := b.CreateFunction("sum", types.I32, 
		[]types.Type{types.I32, types.I32, types.I32, types.I32, types.I32, types.I32, types.I32}, 
		false)
	for i := 0; i < 7; i++ {
		sumFn.Arguments[i].SetName(fmt.Sprintf("arg%d", i))
	}
	
	entry := b.CreateBlock("entry")
	b.SetInsertPoint(entry)
	
	// Sum all arguments
	result := ir.Value(sumFn.Arguments[0])
	for i := 1; i < 7; i++ {
		result = b.CreateAdd(result, sumFn.Arguments[i], fmt.Sprintf("sum%d", i))
	}
	b.CreateRet(result)
	
	// main: call sum(1, 2, 3, 4, 5, 6, 21) = 42
	b.CreateFunction("main", types.I32, nil, false)
	mainEntry := b.CreateBlock("entry")
	b.SetInsertPoint(mainEntry)
	
	args := []ir.Value{
		b.ConstInt(types.I32, 1),
		b.ConstInt(types.I32, 2),
		b.ConstInt(types.I32, 3),
		b.ConstInt(types.I32, 4),
		b.ConstInt(types.I32, 5),
		b.ConstInt(types.I32, 6),
		b.ConstInt(types.I32, 21),
	}
	sumResult := b.CreateCall(sumFn, args, "sum_result")
	b.CreateRet(sumResult)
	
	return m
}

func buildNestedCalls(b *builder.Builder) *ir.Module {
	m := b.CreateModule("nested_calls")
	
	// add(a, b) = a + b
	addFn := b.CreateFunction("add", types.I32, []types.Type{types.I32, types.I32}, false)
	addFn.Arguments[0].SetName("a")
	addFn.Arguments[1].SetName("b")
	entry := b.CreateBlock("entry")
	b.SetInsertPoint(entry)
	sum := b.CreateAdd(addFn.Arguments[0], addFn.Arguments[1], "sum")
	b.CreateRet(sum)
	
	// mul(a, b) = a * b
	mulFn := b.CreateFunction("mul", types.I32, []types.Type{types.I32, types.I32}, false)
	mulFn.Arguments[0].SetName("a")
	mulFn.Arguments[1].SetName("b")
	entry2 := b.CreateBlock("entry")
	b.SetInsertPoint(entry2)
	prod := b.CreateMul(mulFn.Arguments[0], mulFn.Arguments[1], "prod")
	b.CreateRet(prod)
	
	// main: add(mul(2, 3), mul(5, 2)) + 1 = add(6, 10) + 1 = 16 + 1 = 17
	b.CreateFunction("main", types.I32, nil, false)
	mainEntry := b.CreateBlock("entry")
	b.SetInsertPoint(mainEntry)
	
	m1 := b.CreateCall(mulFn, []ir.Value{b.ConstInt(types.I32, 2), b.ConstInt(types.I32, 3)}, "m1")
	m2 := b.CreateCall(mulFn, []ir.Value{b.ConstInt(types.I32, 5), b.ConstInt(types.I32, 2)}, "m2")
	a1 := b.CreateCall(addFn, []ir.Value{m1, m2}, "a1")
	result := b.CreateAdd(a1, b.ConstInt(types.I32, 1), "result")
	b.CreateRet(result)
	
	return m
}

func buildSelect(b *builder.Builder) *ir.Module {
	m := b.CreateModule("select")
	
	b.CreateFunction("main", types.I32, nil, false)
	entry := b.CreateBlock("entry")
	b.SetInsertPoint(entry)
	
	cond := b.CreateICmpSGT(b.ConstInt(types.I32, 10), b.ConstInt(types.I32, 5), "cond")
	result := b.CreateSelect(cond, b.ConstInt(types.I32, 100), b.ConstInt(types.I32, 200), "result")
	b.CreateRet(result)
	
	return m
}

func buildSwitchStatement(b *builder.Builder) *ir.Module {
	m := b.CreateModule("switch_statement")
	
	b.CreateFunction("main", types.I32, nil, false)
	entry := b.CreateBlock("entry")
	case1 := b.CreateBlock("case1")
	case2 := b.CreateBlock("case2")
	case3 := b.CreateBlock("case3")
	defaultCase := b.CreateBlock("default")
	merge := b.CreateBlock("merge")
	
	b.SetInsertPoint(entry)
	value := b.ConstInt(types.I32, 2)
	
	// Fix: Create constant ints separately and cast them
	caseVal1 := b.ConstInt(types.I32, 1)
	caseVal2 := b.ConstInt(types.I32, 2)
	caseVal3 := b.ConstInt(types.I32, 3)
	
	switchInst := &ir.SwitchInst{
		BaseInstruction: ir.BaseInstruction{
			Op:  ir.OpSwitch,
			Ops: []ir.Value{value},
		},
		Condition:    value,
		DefaultBlock: defaultCase,
		Cases: []ir.SwitchCase{
			{Value: caseVal1, Block: case1},
			{Value: caseVal2, Block: case2},
			{Value: caseVal3, Block: case3},
		},
	}
	entry.AddInstruction(switchInst)
	
	b.SetInsertPoint(case1)
	val1 := b.ConstInt(types.I32, 10)
	b.CreateBr(merge)
	
	b.SetInsertPoint(case2)
	val2 := b.ConstInt(types.I32, 30)
	b.CreateBr(merge)
	
	b.SetInsertPoint(case3)
	val3 := b.ConstInt(types.I32, 50)
	b.CreateBr(merge)
	
	b.SetInsertPoint(defaultCase)
	valDefault := b.ConstInt(types.I32, 0)
	b.CreateBr(merge)
	
	b.SetInsertPoint(merge)
	phi := b.CreatePhi(types.I32, "result")
	phi.AddIncoming(val1, case1)
	phi.AddIncoming(val2, case2)
	phi.AddIncoming(val3, case3)
	phi.AddIncoming(valDefault, defaultCase)
	b.CreateRet(phi)
	
	return m
}

func buildMemoryOps(b *builder.Builder) *ir.Module {
	m := b.CreateModule("memory_ops")
	
	b.CreateFunction("main", types.I32, nil, false)
	entry := b.CreateBlock("entry")
	b.SetInsertPoint(entry)
	
	// Allocate space for an i32
	ptr := b.CreateAlloca(types.I32, "ptr")
	
	// Store 99 into it
	b.CreateStore(b.ConstInt(types.I32, 99), ptr)
	
	// Load it back
	loaded := b.CreateLoad(types.I32, ptr, "loaded")
	
	b.CreateRet(loaded)
	
	return m
}

func buildPointerArithmetic(b *builder.Builder) *ir.Module {
	m := b.CreateModule("pointer_arithmetic")
	
	b.CreateFunction("main", types.I32, nil, false)
	entry := b.CreateBlock("entry")
	b.SetInsertPoint(entry)
	
	// Create array of 5 elements: [1, 2, 3, 4, 5]
	arrayType := types.NewArray(types.I32, 5)
	arrayPtr := b.CreateAlloca(arrayType, "array")
	
	// Store values: array[2] = 15
	idx := b.ConstInt(types.I32, 2)
	elemPtr := b.CreateGEP(arrayType, arrayPtr, []ir.Value{b.ConstInt(types.I32, 0), idx}, "elem_ptr")
	b.CreateStore(b.ConstInt(types.I32, 15), elemPtr)
	
	// Load it back
	loaded := b.CreateLoad(types.I32, elemPtr, "loaded")
	
	b.CreateRet(loaded)
	
	return m
}

func buildStructOps(b *builder.Builder) *ir.Module {
	m := b.CreateModule("struct_ops")
	
	// Define struct { i32, i32 }
	structType := types.NewStruct("", []types.Type{types.I32, types.I32}, false)
	
	b.CreateFunction("main", types.I32, nil, false)
	entry := b.CreateBlock("entry")
	b.SetInsertPoint(entry)
	
	// Allocate struct
	structPtr := b.CreateAlloca(structType, "s")
	
	// Get pointer to second field
	field1Ptr := b.CreateGEP(structType, structPtr, 
		[]ir.Value{b.ConstInt(types.I32, 0), b.ConstInt(types.I32, 1)}, "field1_ptr")
	
	// Store 42 in second field
	b.CreateStore(b.ConstInt(types.I32, 42), field1Ptr)
	
	// Load it back
	loaded := b.CreateLoad(types.I32, field1Ptr, "loaded")
	
	b.CreateRet(loaded)
	
	return m
}

func buildArrayOps(b *builder.Builder) *ir.Module {
	m := b.CreateModule("array_ops")
	
	b.CreateFunction("main", types.I32, nil, false)
	entry := b.CreateBlock("entry")
	b.SetInsertPoint(entry)
	
	// Array: [5, 10, 15, 20]
	arrayType := types.NewArray(types.I32, 4)
	arrayPtr := b.CreateAlloca(arrayType, "array")
	
	// Initialize array elements
	for i := 0; i < 4; i++ {
		elemPtr := b.CreateGEP(arrayType, arrayPtr, 
			[]ir.Value{b.ConstInt(types.I32, 0), b.ConstInt(types.I32, int64(i))}, 
			fmt.Sprintf("elem%d_ptr", i))
		b.CreateStore(b.ConstInt(types.I32, int64((i+1)*5)), elemPtr)
	}
	
	// Load element at index 1 (should be 10)
	elem1Ptr := b.CreateGEP(arrayType, arrayPtr, 
		[]ir.Value{b.ConstInt(types.I32, 0), b.ConstInt(types.I32, 1)}, "elem1_ptr")
	result := b.CreateLoad(types.I32, elem1Ptr, "result")
	
	b.CreateRet(result)
	
	return m
}

func buildCastingOps(b *builder.Builder) *ir.Module {
	m := b.CreateModule("casting_ops")
	
	b.CreateFunction("main", types.I32, nil, false)
	entry := b.CreateBlock("entry")
	b.SetInsertPoint(entry)
	
	// Start with i64 value
	val64 := b.ConstInt(types.I64, 42)
	
	// Truncate to i32
	val32 := b.CreateTrunc(val64, types.I32, "val32")
	
	// Extend back to i64
	val64Again := b.CreateSExt(val32, types.I64, "val64_again")
	
	// Truncate back to i32 for return
	result := b.CreateTrunc(val64Again, types.I32, "result")
	
	b.CreateRet(result)
	
	return m
}

func buildComplexPhi(b *builder.Builder) *ir.Module {
	m := b.CreateModule("complex_phi")
	
	b.CreateFunction("main", types.I32, nil, false)
	entry := b.CreateBlock("entry")
	path1 := b.CreateBlock("path1")
	path2 := b.CreateBlock("path2")
	path3 := b.CreateBlock("path3")
	merge := b.CreateBlock("merge")
	
	b.SetInsertPoint(entry)
	selector := b.ConstInt(types.I32, 2)
	
	// Branch based on selector
	cond1 := b.CreateICmpEQ(selector, b.ConstInt(types.I32, 1), "cond1")
	branch1 := b.CreateBlock("branch1")
	b.CreateCondBr(cond1, path1, branch1)
	
	b.SetInsertPoint(branch1)
	cond2 := b.CreateICmpEQ(selector, b.ConstInt(types.I32, 2), "cond2")
	b.CreateCondBr(cond2, path2, path3)
	
	b.SetInsertPoint(path1)
	val1 := b.ConstInt(types.I32, 5)
	b.CreateBr(merge)
	
	b.SetInsertPoint(path2)
	val2 := b.ConstInt(types.I32, 15)
	b.CreateBr(merge)
	
	b.SetInsertPoint(path3)
	val3 := b.ConstInt(types.I32, 25)
	b.CreateBr(merge)
	
	b.SetInsertPoint(merge)
	phi := b.CreatePhi(types.I32, "result")
	phi.AddIncoming(val1, path1)
	phi.AddIncoming(val2, path2)
	phi.AddIncoming(val3, path3)
	b.CreateRet(phi)
	
	return m
}

func buildEarlyReturn(b *builder.Builder) *ir.Module {
	m := b.CreateModule("early_return")
	
	b.CreateFunction("main", types.I32, nil, false)
	entry := b.CreateBlock("entry")
	earlyExit := b.CreateBlock("early_exit")
	normalPath := b.CreateBlock("normal_path")
	
	b.SetInsertPoint(entry)
	value := b.ConstInt(types.I32, 5)
	cond := b.CreateICmpSLT(value, b.ConstInt(types.I32, 10), "cond")
	b.CreateCondBr(cond, earlyExit, normalPath)
	
	b.SetInsertPoint(earlyExit)
	b.CreateRet(value)
	
	b.SetInsertPoint(normalPath)
	b.CreateRet(b.ConstInt(types.I32, 100))
	
	return m
}

func buildMaxFunction(b *builder.Builder) *ir.Module {
	m := b.CreateModule("max_function")
	
	// max(a, b) function
	maxFn := b.CreateFunction("max", types.I32, []types.Type{types.I32, types.I32}, false)
	maxFn.Arguments[0].SetName("a")
	maxFn.Arguments[1].SetName("b")
	
	entry := b.CreateBlock("entry")
	thenBlock := b.CreateBlock("then")
	elseBlock := b.CreateBlock("else")
	merge := b.CreateBlock("merge")
	
	b.SetInsertPoint(entry)
	a := maxFn.Arguments[0]
	b2 := maxFn.Arguments[1]
	cond := b.CreateICmpSGT(a, b2, "cond")
	b.CreateCondBr(cond, thenBlock, elseBlock)
	
	b.SetInsertPoint(thenBlock)
	b.CreateBr(merge)
	
	b.SetInsertPoint(elseBlock)
	b.CreateBr(merge)
	
	b.SetInsertPoint(merge)
	phi := b.CreatePhi(types.I32, "result")
	phi.AddIncoming(a, thenBlock)
	phi.AddIncoming(b2, elseBlock)
	b.CreateRet(phi)
	
	// main function
	b.CreateFunction("main", types.I32, nil, false)
	mainEntry := b.CreateBlock("entry")
	b.SetInsertPoint(mainEntry)
	
	result := b.CreateCall(maxFn, []ir.Value{b.ConstInt(types.I32, 88), b.ConstInt(types.I32, 42)}, "max_result")
	b.CreateRet(result)
	
	return m
}