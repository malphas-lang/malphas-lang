# Verify If Expressions

## Goal Description
Verify that `if` expressions (using `if` as a value) work correctly in Malphas. The code generator is expected to wrap these in IIFEs (Immediately Invoked Function Expressions) in the generated Go code. We need to ensure this generation is correct and handles various cases like nested ifs and side effects.

## User Review Required
None at this stage.

## Proposed Changes
### Verification
#### [NEW] [test_if_expr_comprehensive.mal](file:///Users/daearol/golang_code/malphas-lang-1/examples/test_if_expr_comprehensive.mal)
Create a new test file to cover:
- Basic if/else expressions
- Nested if expressions
- If expressions with side effects in blocks
- If expressions returning different types (should fail type check, but good to verify) - *Wait, type checker should catch this, so maybe not for runtime test*
- If expressions used in variable declarations, function arguments, and return statements.

## Verification Plan
### Automated Tests
- Run `examples/test_if_expr.mal`
- Run `examples/test_if_expr_comprehensive.mal`
- Check generated Go code to ensure IIFEs are correctly formed.
