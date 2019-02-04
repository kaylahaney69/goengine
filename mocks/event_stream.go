// Code generated by mockery v1.0.0. DO NOT EDIT.
package mocks

import goengine_dev "github.com/hellofresh/goengine-dev"
import mock "github.com/stretchr/testify/mock"

// EventStream is an autogenerated mock type for the EventStream type
type EventStream struct {
	mock.Mock
}

// Close provides a mock function with given fields:
func (_m *EventStream) Close() error {
	ret := _m.Called()

	var r0 error
	if rf, ok := ret.Get(0).(func() error); ok {
		r0 = rf()
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// Err provides a mock function with given fields:
func (_m *EventStream) Err() error {
	ret := _m.Called()

	var r0 error
	if rf, ok := ret.Get(0).(func() error); ok {
		r0 = rf()
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// Message provides a mock function with given fields:
func (_m *EventStream) Message() (goengine_dev.Message, int64, error) {
	ret := _m.Called()

	var r0 goengine_dev.Message
	if rf, ok := ret.Get(0).(func() goengine_dev.Message); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(goengine_dev.Message)
		}
	}

	var r1 int64
	if rf, ok := ret.Get(1).(func() int64); ok {
		r1 = rf()
	} else {
		r1 = ret.Get(1).(int64)
	}

	var r2 error
	if rf, ok := ret.Get(2).(func() error); ok {
		r2 = rf()
	} else {
		r2 = ret.Error(2)
	}

	return r0, r1, r2
}

// Next provides a mock function with given fields:
func (_m *EventStream) Next() bool {
	ret := _m.Called()

	var r0 bool
	if rf, ok := ret.Get(0).(func() bool); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(bool)
	}

	return r0
}
