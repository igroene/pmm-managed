// Code generated by mockery v1.0.0. DO NOT EDIT.

package server

import (
	context "context"

	mock "github.com/stretchr/testify/mock"

	models "github.com/percona/pmm-managed/models"

	scheduler "github.com/percona/pmm-managed/services/scheduler"

	time "time"
)

// mockScheduleService is an autogenerated mock type for the scheduleService type
type mockScheduleService struct {
	mock.Mock
}

// Add provides a mock function with given fields: task, enabled, cronExpr, startAt, retry, retryInterval
func (_m *mockScheduleService) Add(task scheduler.Task, enabled bool, cronExpr string, startAt time.Time, retry uint, retryInterval time.Duration) (*models.ScheduledTask, error) {
	ret := _m.Called(task, enabled, cronExpr, startAt, retry, retryInterval)

	var r0 *models.ScheduledTask
	if rf, ok := ret.Get(0).(func(scheduler.Task, bool, string, time.Time, uint, time.Duration) *models.ScheduledTask); ok {
		r0 = rf(task, enabled, cronExpr, startAt, retry, retryInterval)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*models.ScheduledTask)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(scheduler.Task, bool, string, time.Time, uint, time.Duration) error); ok {
		r1 = rf(task, enabled, cronExpr, startAt, retry, retryInterval)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// Remove provides a mock function with given fields: id
func (_m *mockScheduleService) Remove(id string) error {
	ret := _m.Called(id)

	var r0 error
	if rf, ok := ret.Get(0).(func(string) error); ok {
		r0 = rf(id)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// Run provides a mock function with given fields: ctx
func (_m *mockScheduleService) Run(ctx context.Context) {
	_m.Called(ctx)
}