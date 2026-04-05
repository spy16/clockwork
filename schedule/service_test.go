package schedule_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/spy16/clockwork"
	"github.com/spy16/clockwork/client"
	"github.com/spy16/clockwork/schedule"
	"github.com/spy16/clockwork/schedule/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestService_List(t *testing.T) {
	sc := mocks.NewScheduler(t)
	sc.EXPECT().List(mock.Anything, 0, 10).Return([]schedule.Schedule{}, nil)

	svc := schedule.Service{Scheduler: sc}

	got, err := svc.List(context.Background(), 0, 10)
	assert.NoError(t, err)
	assert.NotNil(t, got)
	assert.Len(t, got, 0)
}

func TestService_Fetch(t *testing.T) {
	t.Parallel()

	t.Run("ScheduleNotFound", func(t *testing.T) {
		mockSched := mocks.NewScheduler(t)
		mockSched.EXPECT().Get(mock.Anything, "foo").Return(nil, clockwork.ErrNotFound)

		svc := schedule.Service{Scheduler: mockSched}

		got, err := svc.Fetch(context.Background(), "foo")
		assert.True(t, errors.Is(err, clockwork.ErrNotFound), "expected ErrNotFound, got %v", err)
		assert.Nil(t, got)
	})

	t.Run("ScheduleFoundButNoAccess", func(t *testing.T) {
		mockSched := mocks.NewScheduler(t)
		mockSched.EXPECT().
			Get(mock.Anything, "foo").
			Return(&schedule.Schedule{ID: "foo"}, nil)

		mockClientReg := mocks.NewClientRegistry(t)
		mockClientReg.EXPECT().
			IsAdmin(mock.Anything, "current-client").
			Return(false)

		svc := schedule.Service{Scheduler: mockSched, Clients: mockClientReg}

		ctx := client.Context(context.Background(), client.Client{ID: "current-client"})
		got, err := svc.Fetch(ctx, "foo")
		assert.True(t, errors.Is(err, clockwork.ErrUnauthorized))
		assert.Nil(t, got)
	})

	t.Run("ScheduleFoundAndAccessibleAsAdmin", func(t *testing.T) {
		mockSched := mocks.NewScheduler(t)
		mockSched.EXPECT().
			Get(mock.Anything, "foo").
			Return(&schedule.Schedule{ID: "foo", ClientID: "some-client"}, nil)

		mockClientReg := mocks.NewClientRegistry(t)
		mockClientReg.EXPECT().
			IsAdmin(mock.Anything, "current-client").
			Return(true) // current client is an admin

		svc := schedule.Service{Scheduler: mockSched, Clients: mockClientReg}

		ctx := client.Context(context.Background(), client.Client{ID: "current-client"})
		got, err := svc.Fetch(ctx, "foo")
		assert.NoError(t, err)
		assert.Equal(t, got, &schedule.Schedule{
			ID:       "foo",
			ClientID: "some-client",
		})
	})

	t.Run("ScheduleFoundAndAccessibleAsOwner", func(t *testing.T) {
		mockSched := mocks.NewScheduler(t)
		mockSched.EXPECT().
			Get(mock.Anything, "foo").
			Return(&schedule.Schedule{ID: "foo", ClientID: "some-client"}, nil)

		mockClientReg := mocks.NewClientRegistry(t)
		mockClientReg.EXPECT().
			IsAdmin(mock.Anything, "some-client").
			Return(false) // current client is not admin

		svc := schedule.Service{Scheduler: mockSched, Clients: mockClientReg}

		ctx := client.Context(context.Background(), client.Client{ID: "some-client"})
		got, err := svc.Fetch(ctx, "foo")
		assert.NoError(t, err)
		assert.Equal(t, got, &schedule.Schedule{
			ID:       "foo",
			ClientID: "some-client",
		})
	})

}

func TestService_Create(t *testing.T) {
	t.Parallel()

	frozenTime := time.Unix(1697439139, 0).UTC()

	pastSchedule := schedule.Schedule{
		ID:       "past-schedule",
		Crontab:  "@at 1697439120", // this is in past compared to frozenTime.
		ClientID: "sample-client",
		Payload:  "this is in past",
		Category: "test",
	}

	futureSchedule := schedule.Schedule{
		ID:       "future-schedule",
		Crontab:  "@at 1697439140", // this is in future compared to frozenTime.
		ClientID: "sample-client",
		Payload:  "this is in future",
		Category: "test",
	}

	tests := []struct {
		title   string
		sc      schedule.Schedule
		setup   func(t *testing.T) *schedule.Service
		want    *schedule.Schedule
		wantErr string
	}{
		{
			title: "InvalidSchedule",
			sc:    schedule.Schedule{},
			setup: func(t *testing.T) *schedule.Service {
				return &schedule.Service{}
			},
			wantErr: "bad_request: id must not be empty",
		},
		{
			title: "NonExistentClient",
			sc:    futureSchedule,
			setup: func(t *testing.T) *schedule.Service {
				mockClient := mocks.NewClientRegistry(t)
				mockClient.EXPECT().
					GetClient(mock.Anything, "sample-client").
					Return(nil, clockwork.ErrNotFound.WithMsgf("client not found"))

				return &schedule.Service{
					Clock:   func() time.Time { return time.Unix(1697439139, 0) },
					Clients: mockClient,
				}
			},
			wantErr: "not_found: client not found",
		},
		{
			title: "SchedulerPutFailure",
			sc:    futureSchedule,
			setup: func(t *testing.T) *schedule.Service {
				mockClient := mocks.NewClientRegistry(t)
				mockClient.EXPECT().
					GetClient(mock.Anything, "sample-client").
					Return(&client.Client{ID: "sample-client"}, nil)

				mockScheduler := mocks.NewScheduler(t)
				mockScheduler.EXPECT().
					Put(mock.Anything, mock.Anything, false, mock.Anything).
					Return(clockwork.ErrInternal.WithMsgf("scheduler put failed"))

				return &schedule.Service{
					Clock:     func() time.Time { return time.Unix(1697439139, 0) },
					Clients:   mockClient,
					Scheduler: mockScheduler,
				}
			},
			wantErr: "internal_error: scheduler put failed",
		},
		{
			title: "Success_WithPastTimestamp",
			sc:    pastSchedule,
			setup: func(t *testing.T) *schedule.Service {
				mockClient := mocks.NewClientRegistry(t)
				mockClient.EXPECT().
					GetClient(mock.Anything, "sample-client").
					Return(&client.Client{ID: "sample-client"}, nil)

				mockScheduler := mocks.NewScheduler(t)
				mockScheduler.EXPECT().
					Put(mock.Anything, mock.Anything, false, schedule.Execution{
						Manual:     false,
						Version:    0,
						EnqueueAt:  frozenTime,
						ScheduleID: "past-schedule",
					}).
					Return(nil)

				mockChangeLog := mocks.NewChangeLogger(t)
				mockChangeLog.EXPECT().
					Publish(mock.Anything, mock.Anything, mock.Anything).
					Return(nil)

				return &schedule.Service{
					Clock:     func() time.Time { return frozenTime },
					Clients:   mockClient,
					Changes:   mockChangeLog,
					Scheduler: mockScheduler,
				}
			},
			want: &schedule.Schedule{
				ID:              "past-schedule",
				Status:          schedule.StatusActive,
				Crontab:         "@at 1697439120", // this is in past compared to frozenTime.
				Category:        "test",
				Payload:         "this is in past",
				ClientID:        "sample-client",
				CreatedAt:       frozenTime,
				UpdatedAt:       frozenTime,
				EnqueueCount:    1,
				NextExecutionAt: time.Unix(1697439139, 0),
			},
		},
		{
			title: "Success_WithFutureTimestamp",
			sc:    futureSchedule,
			setup: func(t *testing.T) *schedule.Service {
				mockClient := mocks.NewClientRegistry(t)
				mockClient.EXPECT().
					GetClient(mock.Anything, "sample-client").
					Return(&client.Client{ID: "sample-client"}, nil)

				mockScheduler := mocks.NewScheduler(t)
				mockScheduler.EXPECT().
					Put(
						mock.Anything,
						schedule.Schedule{
							ID:              "future-schedule",
							Status:          schedule.StatusActive,
							Crontab:         "@at 1697439140", // this is in future compared to frozenTime.
							Category:        "test",
							Payload:         "this is in future",
							ClientID:        "sample-client",
							CreatedAt:       frozenTime,
							UpdatedAt:       frozenTime,
							Version:         0,
							EnqueueCount:    1,
							NextExecutionAt: time.Unix(1697439140, 0),
						},
						false,
						schedule.Execution{
							Manual:     false,
							Version:    0,
							EnqueueAt:  time.Unix(1697439140, 0),
							ScheduleID: "future-schedule",
						}).
					Return(nil)

				mockChangeLog := mocks.NewChangeLogger(t)
				mockChangeLog.EXPECT().
					Publish(mock.Anything, mock.Anything, mock.Anything).
					Return(nil)

				return &schedule.Service{
					Clock:     func() time.Time { return frozenTime },
					Clients:   mockClient,
					Changes:   mockChangeLog,
					Scheduler: mockScheduler,
				}
			},
			want: &schedule.Schedule{
				ID:              "future-schedule",
				Status:          schedule.StatusActive,
				Crontab:         "@at 1697439140",
				Category:        "test",
				Payload:         "this is in future",
				ClientID:        "sample-client",
				CreatedAt:       frozenTime,
				UpdatedAt:       frozenTime,
				EnqueueCount:    1,
				NextExecutionAt: time.Unix(1697439140, 0),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.title, func(t *testing.T) {
			svc := tt.setup(t)

			got, err := svc.Create(context.Background(), tt.sc)
			if tt.wantErr != "" {
				assert.ErrorContains(t, err, tt.wantErr)
				assert.Nil(t, got)
			} else {
				fmt.Println(got.NextExecutionAt.Unix())
				assert.NoError(t, err)
				assert.True(t, cmp.Equal(tt.want, got), cmp.Diff(tt.want, got))
			}
		})
	}
}

func TestService_Update(t *testing.T) {
	t.Parallel()

	frozenTime := time.Unix(1697439139, 0).UTC()

	sampleActiveSchedule := schedule.Schedule{
		ID:              "past-schedule",
		Status:          schedule.StatusActive,
		Crontab:         "@at 1697439120", // this is in past compared to frozenTime.
		Category:        "test",
		Payload:         "this is in past",
		ClientID:        "sample-client",
		CreatedAt:       frozenTime,
		UpdatedAt:       frozenTime,
		EnqueueCount:    1,
		NextExecutionAt: frozenTime,
	}

	tests := []struct {
		title         string
		setup         func(t *testing.T) *schedule.Service
		currentClient string
		scheduleID    string
		updates       schedule.Updates
		want          *schedule.Schedule
		wantErr       string
	}{
		{
			title: "ScheduleNotFound",
			setup: func(t *testing.T) *schedule.Service {
				mockScheduler := mocks.NewScheduler(t)
				mockScheduler.EXPECT().
					Get(mock.Anything, "foo").
					Return(nil, clockwork.ErrNotFound.WithMsgf("schedule not found"))

				return &schedule.Service{
					Scheduler: mockScheduler,
				}
			},
			scheduleID: "foo",
			updates:    schedule.Updates{},
			wantErr:    "not_found: schedule not found",
		},
		{
			title: "ClientNotFound",
			setup: func(t *testing.T) *schedule.Service {
				mockScheduler := mocks.NewScheduler(t)
				mockScheduler.EXPECT().
					Get(mock.Anything, "foo").
					Return(&sampleActiveSchedule, nil)

				mockClientReg := mocks.NewClientRegistry(t)
				mockClientReg.EXPECT().
					GetClient(mock.Anything, "sample-client").
					Return(nil, clockwork.ErrNotFound.WithMsgf("client not found"))

				return &schedule.Service{
					Clients:   mockClientReg,
					Scheduler: mockScheduler,
				}
			},
			scheduleID: "foo",
			updates:    schedule.Updates{},
			wantErr:    "not_found: client not found",
		},
		{
			title: "ClientNotAuthorized",
			setup: func(t *testing.T) *schedule.Service {
				mockScheduler := mocks.NewScheduler(t)
				mockScheduler.EXPECT().
					Get(mock.Anything, "foo").
					Return(&sampleActiveSchedule, nil)

				mockClientReg := mocks.NewClientRegistry(t)
				mockClientReg.EXPECT().
					GetClient(mock.Anything, "sample-client").
					Return(&client.Client{ID: "sample-client"}, nil)

				mockClientReg.EXPECT().
					IsAdmin(mock.Anything, "some-other-client").
					Return(false)

				return &schedule.Service{
					Clients:   mockClientReg,
					Scheduler: mockScheduler,
				}
			},
			currentClient: "some-other-client",
			scheduleID:    "foo",
			updates:       schedule.Updates{},
			wantErr:       "unauthorized: not authorised to access this schedule",
		},
		{
			title: "InvalidUpdates",
			setup: func(t *testing.T) *schedule.Service {
				mockScheduler := mocks.NewScheduler(t)
				mockScheduler.EXPECT().
					Get(mock.Anything, "foo").
					Return(&sampleActiveSchedule, nil)

				mockClientReg := mocks.NewClientRegistry(t)
				mockClientReg.EXPECT().
					GetClient(mock.Anything, "sample-client").
					Return(&client.Client{ID: "sample-client"}, nil)

				mockClientReg.EXPECT().
					IsAdmin(mock.Anything, "some-other-client").
					Return(true)

				return &schedule.Service{
					Clock:     func() time.Time { return frozenTime },
					Clients:   mockClientReg,
					Scheduler: mockScheduler,
				}
			},
			currentClient: "some-other-client",
			scheduleID:    "foo",
			updates: schedule.Updates{
				Crontab: "@at invalid-time",
			},
			wantErr: "bad_request: invalid crontab",
		},
		{
			title: "UpdateToPastTimestamp",
			setup: func(t *testing.T) *schedule.Service {
				mockScheduler := mocks.NewScheduler(t)
				mockScheduler.EXPECT().
					Get(mock.Anything, "foo").
					Return(&sampleActiveSchedule, nil)

				mockScheduler.EXPECT().
					Put(mock.Anything, mock.Anything, true, mock.Anything).
					Return(nil)

				mockClientReg := mocks.NewClientRegistry(t)
				mockClientReg.EXPECT().
					GetClient(mock.Anything, "sample-client").
					Return(&client.Client{ID: "sample-client"}, nil)

				mockClientReg.EXPECT().
					IsAdmin(mock.Anything, "some-other-client").
					Return(true)

				return &schedule.Service{
					Clock: func() time.Time {
						// let's say update was done after 10 minutes from creation.
						return frozenTime.Add(10 * time.Minute)
					},
					Clients:   mockClientReg,
					Scheduler: mockScheduler,
				}
			},
			currentClient: "some-other-client",
			scheduleID:    "foo",
			updates: schedule.Updates{
				Crontab: "@at 1697439120",
			},
			want: &schedule.Schedule{
				ID:              "past-schedule",
				Status:          schedule.StatusActive,
				Crontab:         "@at 1697439120",
				Version:         2,
				Category:        "test",
				Payload:         "this is in past",
				ClientID:        "sample-client",
				CreatedAt:       frozenTime,
				UpdatedAt:       frozenTime.Add(10 * time.Minute),
				EnqueueCount:    1,
				NextExecutionAt: frozenTime,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.title, func(t *testing.T) {
			svc := tt.setup(t)

			if tt.currentClient == "" {
				tt.currentClient = "sample-client"
			}
			ctx := client.Context(context.Background(), client.Client{ID: tt.currentClient})

			got, err := svc.Update(ctx, tt.scheduleID, tt.updates)
			if tt.wantErr != "" {
				assert.ErrorContains(t, err, tt.wantErr)
				assert.Nil(t, got)
			} else {
				assert.NoError(t, err)
				assert.True(t, cmp.Equal(tt.want, got), cmp.Diff(tt.want, got))
			}
		})
	}
}
