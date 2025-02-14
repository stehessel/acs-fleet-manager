// Code generated by moq; DO NOT EDIT.
// github.com/matryer/moq

package probe

import (
	"context"
	"sync"
)

// Ensure, that ProbeMock does implement Probe.
// If this is not the case, regenerate this file with moq.
var _ Probe = &ProbeMock{}

// ProbeMock is a mock implementation of Probe.
//
//	func TestSomethingThatUsesProbe(t *testing.T) {
//
//		// make and configure a mocked Probe
//		mockedProbe := &ProbeMock{
//			CleanUpFunc: func(ctx context.Context) error {
//				panic("mock out the CleanUp method")
//			},
//			ExecuteFunc: func(ctx context.Context) error {
//				panic("mock out the Execute method")
//			},
//		}
//
//		// use mockedProbe in code that requires Probe
//		// and then make assertions.
//
//	}
type ProbeMock struct {
	// CleanUpFunc mocks the CleanUp method.
	CleanUpFunc func(ctx context.Context) error

	// ExecuteFunc mocks the Execute method.
	ExecuteFunc func(ctx context.Context) error

	// calls tracks calls to the methods.
	calls struct {
		// CleanUp holds details about calls to the CleanUp method.
		CleanUp []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
		}
		// Execute holds details about calls to the Execute method.
		Execute []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
		}
	}
	lockCleanUp sync.RWMutex
	lockExecute sync.RWMutex
}

// CleanUp calls CleanUpFunc.
func (mock *ProbeMock) CleanUp(ctx context.Context) error {
	if mock.CleanUpFunc == nil {
		panic("ProbeMock.CleanUpFunc: method is nil but Probe.CleanUp was just called")
	}
	callInfo := struct {
		Ctx context.Context
	}{
		Ctx: ctx,
	}
	mock.lockCleanUp.Lock()
	mock.calls.CleanUp = append(mock.calls.CleanUp, callInfo)
	mock.lockCleanUp.Unlock()
	return mock.CleanUpFunc(ctx)
}

// CleanUpCalls gets all the calls that were made to CleanUp.
// Check the length with:
//
//	len(mockedProbe.CleanUpCalls())
func (mock *ProbeMock) CleanUpCalls() []struct {
	Ctx context.Context
} {
	var calls []struct {
		Ctx context.Context
	}
	mock.lockCleanUp.RLock()
	calls = mock.calls.CleanUp
	mock.lockCleanUp.RUnlock()
	return calls
}

// Execute calls ExecuteFunc.
func (mock *ProbeMock) Execute(ctx context.Context) error {
	if mock.ExecuteFunc == nil {
		panic("ProbeMock.ExecuteFunc: method is nil but Probe.Execute was just called")
	}
	callInfo := struct {
		Ctx context.Context
	}{
		Ctx: ctx,
	}
	mock.lockExecute.Lock()
	mock.calls.Execute = append(mock.calls.Execute, callInfo)
	mock.lockExecute.Unlock()
	return mock.ExecuteFunc(ctx)
}

// ExecuteCalls gets all the calls that were made to Execute.
// Check the length with:
//
//	len(mockedProbe.ExecuteCalls())
func (mock *ProbeMock) ExecuteCalls() []struct {
	Ctx context.Context
} {
	var calls []struct {
		Ctx context.Context
	}
	mock.lockExecute.RLock()
	calls = mock.calls.Execute
	mock.lockExecute.RUnlock()
	return calls
}
