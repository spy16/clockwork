package clockwork_test

import (
	goerrors "errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/spy16/clockwork"
)

func TestError_Error(t *testing.T) {
	t.Parallel()

	table := []struct {
		title string
		err   clockwork.Error
		want  string
	}{
		{
			title: "WithoutCause",
			err:   clockwork.ErrInvalid,
			want:  "bad_request: request is not valid",
		},
		{
			title: "WithCause",
			err:   clockwork.ErrInvalid.WithCausef("foo"),
			want:  "bad_request: foo",
		},
	}

	for _, tt := range table {
		t.Run(tt.title, func(t *testing.T) {
			got := tt.err.Error()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestError_Is(t *testing.T) {
	t.Parallel()

	table := []struct {
		title string
		err   clockwork.Error
		other error
		want  bool
	}{
		{
			title: "NonClockworkErr",
			err:   clockwork.ErrInternal,
			other: goerrors.New("foo"),
			want:  false,
		},
		{
			title: "ClockworkErrWithDifferentCode",
			err:   clockwork.ErrInternal,
			other: clockwork.ErrInvalid,
			want:  false,
		},
		{
			title: "ClockworkErrWithSameCodeDiffCause",
			err:   clockwork.ErrInvalid.WithCausef("cause 1"),
			other: clockwork.ErrInvalid.WithCausef("cause 2"),
			want:  true,
		},
	}

	for _, tt := range table {
		t.Run(tt.title, func(t *testing.T) {
			got := goerrors.Is(tt.err, tt.other)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestError_WithCausef(t *testing.T) {
	t.Parallel()

	table := []struct {
		title string
		err   clockwork.Error
		want  clockwork.Error
	}{
		{
			title: "WithCauseString",
			err:   clockwork.ErrInvalid.WithCausef("foo"),
			want: clockwork.Error{
				Code:    "bad_request",
				Message: "Request is not valid",
				Cause:   "foo",
			},
		},
		{
			title: "WithCauseFormatted",
			err:   clockwork.ErrInvalid.WithCausef("hello %s", "world"),
			want: clockwork.Error{
				Code:    "bad_request",
				Message: "Request is not valid",
				Cause:   "hello world",
			},
		},
	}

	for _, tt := range table {
		t.Run(tt.title, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.err)
		})
	}
}

func TestError_WithMsgf(t *testing.T) {
	t.Parallel()

	table := []struct {
		title string
		err   clockwork.Error
		want  clockwork.Error
	}{
		{
			title: "WithCauseString",
			err:   clockwork.ErrInvalid.WithMsgf("foo"),
			want: clockwork.Error{
				Code:    "bad_request",
				Message: "foo",
			},
		},
		{
			title: "WithCauseFormatted",
			err:   clockwork.ErrInvalid.WithMsgf("hello %s", "world"),
			want: clockwork.Error{
				Code:    "bad_request",
				Message: "hello world",
			},
		},
	}

	for _, tt := range table {
		t.Run(tt.title, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.err)
		})
	}
}

func Test_Errorf(t *testing.T) {
	e := clockwork.Errorf("failed: %d", 100)
	assert.Error(t, e)
	assert.EqualError(t, e, "internal_error: failed: 100")
}
