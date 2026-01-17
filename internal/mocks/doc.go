// Package mocks provides mock implementations for testing.
//
// This package contains:
//   - Interface definitions for mockable dependencies
//   - Mock implementations for gRPC clients
//   - Mock implementations for database operations
//   - Test helpers and fixtures
//
// Usage:
//
//	func TestSomething(t *testing.T) {
//	    mock := mocks.NewMockFunctionStore()
//	    mock.SetReturnValue("function1", expectedResult)
//	    // use mock in test
//	}
package mocks
