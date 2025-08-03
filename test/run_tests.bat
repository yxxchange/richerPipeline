@echo off
echo ====================================
echo PipeFree Integration Test Suite
echo ====================================
echo.

REM 检查 Go 环境
go version >nul 2>&1
if errorlevel 1 (
    echo Error: Go is not installed or not in PATH
    exit /b 1
)

REM 设置测试环境变量
set CGO_ENABLED=1
set GO111MODULE=on

echo [1/5] Building project...
go build ./... 
if errorlevel 1 (
    echo Error: Build failed
    exit /b 1
)
echo ✓ Build successful

echo.
echo [2/5] Running basic integration tests...
go test ./test/ -run TestServerClientInteraction -v -timeout=60s
if errorlevel 1 (
    echo ✗ Basic integration test failed
    exit /b 1
)
echo ✓ Basic integration test passed

echo.
echo [3/5] Running resilience tests...
go test ./test/ -run TestClientResilience -v -timeout=30s
if errorlevel 1 (
    echo ✗ Resilience test failed
    exit /b 1
)
echo ✓ Resilience test passed

echo.
echo [4/5] Running concurrent client tests...
go test ./test/ -run TestConcurrentClients -v -timeout=30s
if errorlevel 1 (
    echo ✗ Concurrent client test failed
    exit /b 1
)
echo ✓ Concurrent client test passed

echo.
echo [5/5] Running edge cases tests...
go test ./test/ -run TestEdgeCases -v -timeout=30s
if errorlevel 1 (
    echo ✗ Edge cases test failed
    exit /b 1
)
echo ✓ Edge cases test passed

echo.
echo ====================================
echo All Core Tests Passed! 🎉
echo ====================================
echo.

REM 询问是否运行性能测试
set /p run_perf="Run performance tests? (y/n): "
if /i "%run_perf%"=="y" (
    echo.
    echo Running performance tests...
    go test ./test/ -run TestHighConcurrencyEvents -v -timeout=60s
    go test ./test/ -run TestCacheConsistency -v -timeout=30s
    go test ./test/ -run TestNetworkPartition -v -timeout=30s
    echo ✓ Performance tests completed
)

REM 询问是否运行基准测试
set /p run_bench="Run benchmark tests? (y/n): "
if /i "%run_bench%"=="y" (
    echo.
    echo Running benchmark tests...
    go test ./test/ -bench=BenchmarkInformerThroughput -benchtime=10s -v
    go test ./test/ -bench=BenchmarkCacheQuery -benchtime=5s -v
    echo ✓ Benchmark tests completed
)

echo.
echo ====================================
echo Test Suite Completed Successfully!
echo ====================================
echo.
echo Summary:
echo - Core integration tests: PASSED
echo - Resilience tests: PASSED  
echo - Concurrent client tests: PASSED
echo - Edge cases tests: PASSED
if /i "%run_perf%"=="y" echo - Performance tests: COMPLETED
if /i "%run_bench%"=="y" echo - Benchmark tests: COMPLETED
echo.
echo The PipeFree client_pipe-server interaction is working correctly! 🚀
echo.
echo Note: Make sure etcd and database are running before executing tests.
echo You can run individual tests with: go test ./test/ -run TestName -v