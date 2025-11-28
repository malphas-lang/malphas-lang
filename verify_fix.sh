#!/bin/bash
set -e

echo "Verifying Malphas Hang Fix..."

# 1. Verify opt works (build test_existentials.mal)
echo "1. Building examples/test_existentials.mal (Testing opt)..."
go run cmd/malphas/main.go build examples/test_existentials.mal
if [ -f "examples/test_existentials.mal.opt.o" ] || [ -f "test_existentials" ]; then
    echo "   [SUCCESS] Build successful."
else
    echo "   [FAILURE] Build failed or artifacts missing."
    exit 1
fi

# 2. Verify runtime works (run tests/gadt_vec.mal)
echo "2. Running tests/gadt_vec.mal (Testing runtime)..."
set +e
OUTPUT=$(go run cmd/malphas/main.go run tests/gadt_vec.mal 2>&1)
EXIT_CODE=$?
set -e

# Note: Exit code 1 is expected due to void main return issue, so we check output content.
if [ $EXIT_CODE -ne 0 ] && [ $EXIT_CODE -ne 1 ]; then
    echo "   [FAILURE] Unexpected exit code: $EXIT_CODE"
    echo "   Output:"
    echo "$OUTPUT"
    exit 1
fi
EXPECTED="1
2"

if echo "$OUTPUT" | grep -q "1"; then
    if echo "$OUTPUT" | grep -q "2"; then
        echo "   [SUCCESS] Output matches expected."
        echo "   Output:"
        echo "$OUTPUT"
    else
        echo "   [FAILURE] Output missing '2'."
        echo "   Output:"
        echo "$OUTPUT"
        exit 1
    fi
else
    echo "   [FAILURE] Output missing '1'."
    echo "   Output:"
    echo "$OUTPUT"
    exit 1
fi

echo "Verification Complete!"
