// Code generated by mockery v1.0.0. DO NOT EDIT.

package scheduler

import (
	context "context"

	mock "github.com/stretchr/testify/mock"
)

// mockBackupsLogicService is an autogenerated mock type for the backupsLogicService type
type mockBackupsLogicService struct {
	mock.Mock
}

// PerformBackup provides a mock function with given fields: ctx, serviceID, locationID, name
func (_m *mockBackupsLogicService) PerformBackup(ctx context.Context, serviceID string, locationID string, name string) (string, error) {
	ret := _m.Called(ctx, serviceID, locationID, name)

	var r0 string
	if rf, ok := ret.Get(0).(func(context.Context, string, string, string) string); ok {
		r0 = rf(ctx, serviceID, locationID, name)
	} else {
		r0 = ret.Get(0).(string)
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, string, string, string) error); ok {
		r1 = rf(ctx, serviceID, locationID, name)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}